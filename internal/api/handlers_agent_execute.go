package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"promptsheon/internal/models"
	"promptsheon/internal/trace"
	"promptsheon/internal/workflow"
)

func (s *Server) handleExecuteAgent(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	agent, err := s.db.GetAgent(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}
	if agent.Status != models.StatusApproved && agent.Status != models.StatusDeployed {
		return badRequest("can only execute approved or deployed agents")
	}

	var req struct {
		Input     map[string]any `json:"input"`
		ContextID string         `json:"context_id,omitempty"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.Input == nil {
		req.Input = make(map[string]any)
	}

	// Add timeout to prevent indefinite execution
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	start := time.Now()

	// Create trace span
	var span *trace.Span
	if s.spans != nil {
		span = s.spans.Start(r.Context(), "agent.execute")
		span.SetAttribute("agent_id", agent.ID)
		span.SetAttribute("agent_name", agent.Name)
		span.SetAttribute("steps", fmt.Sprintf("%d", len(agent.Steps)))
	}

	// Load guardrail config if agent has one
	var agentGuardrailCfg *models.AgentGuardrailConfig
	if agent.GuardrailConfigID != "" {
		cfg, err := s.db.GetAgentGuardrailConfig(ctx, agent.GuardrailConfigID)
		if err == nil {
			agentGuardrailCfg = cfg
		}
	} else {
		// Also check by agent ID
		cfg, err := s.db.GetAgentGuardrailConfigByAgent(ctx, agent.ID)
		if err == nil {
			agentGuardrailCfg = cfg
		}
	}

	// Pre-execution guardrail checks
	if s.guardrailManager != nil && agentGuardrailCfg != nil && agentGuardrailCfg.Enabled {
		// Build a summary of the input for guardrail checking
		inputSummary := fmt.Sprintf("agent:%s input:%v", agent.Name, req.Input)
		violations := s.guardrailManager.RunAgentChecks(ctx, agentGuardrailCfg.RestrictedTerms, agentGuardrailCfg.ContentPolicy, inputSummary)
		if len(violations) > 0 && agentGuardrailCfg.StopOnViolation {
			if span != nil {
				span.SetAttribute("guardrail_blocked", "true")
				span.SetError(fmt.Errorf("guardrail violation: %d violations detected", len(violations)))
				span.Finish()
				s.spans.Finish(span)
			}
			return badRequestf("guardrail violation: %s", violations[0].Message)
		}
	}

	// Create workflow engine
	registry := workflow.DefaultRegistry()
	engine := workflow.NewEngine(registry)

	// Configure guardrails on engine
	if s.guardrailManager != nil && agentGuardrailCfg != nil && agentGuardrailCfg.Enabled {
		engine.SetGuardrails(s.guardrailManager, agentGuardrailCfg)
	}

	// Configure context manager on engine
	if s.contextManager != nil {
		engine.SetContextManager(s.contextManager)
	}

	// Execute workflow
	result, err := engine.Execute(ctx, agent, req.Input)
	latency := time.Since(start)

	if err != nil {
		if span != nil {
			span.SetError(err)
			span.Finish()
			s.spans.Finish(span)
		}
		return err
	}

	// Post-execution guardrail checks on output
	var postViolations []string
	if s.guardrailManager != nil && agentGuardrailCfg != nil && agentGuardrailCfg.Enabled {
		for _, stepResult := range result.Steps {
			if stepResult.Output != nil {
				// Check all string values in step output
				for _, v := range stepResult.Output {
					if str, ok := v.(string); ok {
						if vr := s.guardrailManager.CheckContentPolicy(str, agentGuardrailCfg.ContentPolicy); !vr.Passed {
							s.guardrailManager.RecordViolation(vr.Violation)
							postViolations = append(postViolations, vr.Violation.Message)
						}
					}
				}
				// Also check tool call outputs
				for _, tc := range stepResult.ToolCalls {
					if tc.Output != nil {
						for _, v := range tc.Output {
							if str, ok := v.(string); ok {
								if vr := s.guardrailManager.CheckContentPolicy(str, agentGuardrailCfg.ContentPolicy); !vr.Passed {
									s.guardrailManager.RecordViolation(vr.Violation)
									postViolations = append(postViolations, vr.Violation.Message)
								}
							}
						}
					}
				}
			}
		}
	}

	// Calculate cost
	var totalCost float64
	if s.collector != nil {
		totalCost = 0 // TODO: integrate with LLM cost calculation per step
	}

	// Build execution steps
	execSteps := make([]models.AgentExecutionStep, 0, len(result.Steps))
	for stepID, sr := range result.Steps {
		execSteps = append(execSteps, models.AgentExecutionStep{
			StepID:    stepID,
			Status:    string(sr.Status),
			Output:    sr.Output,
			Error:     sr.Error,
			ToolCalls: sr.ToolCalls,
			LatencyMs: sr.LatencyMs,
		})
	}

	now := time.Now()
	execution := &models.AgentExecution{
		ID:                  generateID(),
		AgentID:             agent.ID,
		WorkflowID:          result.WorkflowID,
		Status:              string(result.Status),
		Input:               req.Input,
		Output:              result.Outputs,
		Steps:               execSteps,
		TotalCostUSD:        totalCost,
		TotalLatencyMs:      latency.Milliseconds(),
		GuardrailViolations: postViolations,
		ContextID:           req.ContextID,
		CreatedAt:           now,
		CompletedAt:         &now,
	}

	// Persist execution
	if err := s.db.SaveAgentExecution(ctx, execution); err != nil {
		s.logger.Error("failed to save agent execution", "err", err, "agent_id", agent.ID)
	}

	// Record metrics
	if s.collector != nil {
		s.collector.AgentExecutionsTotal.Inc()
		s.collector.AgentExecutionLatency.Observe(latency.Seconds())
	}

	// Finish trace
	if span != nil {
		span.SetAttribute("execution_id", execution.ID)
		span.SetAttribute("status", string(result.Status))
		span.SetAttribute("latency_ms", fmt.Sprintf("%d", latency.Milliseconds()))
		span.SetAttribute("total_cost_usd", fmt.Sprintf("%.6f", totalCost))
		if len(postViolations) > 0 {
			span.SetAttribute("guardrail_violations", fmt.Sprintf("%d", len(postViolations)))
		}
		span.Finish()
		s.spans.Finish(span)
	}

	// Audit log
	s.audit(r.Context(), "agent_execute", "agent:"+agent.ID, map[string]any{
		"execution_id": execution.ID,
		"status":       result.Status,
		"steps":        len(result.Steps),
		"latency_ms":   latency.Milliseconds(),
		"cost_usd":     totalCost,
	})

	writeJSON(w, http.StatusOK, execution)
	return nil
}
