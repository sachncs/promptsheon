package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/sachn-cs/promptsheon/internal/models"
	"github.com/sachn-cs/promptsheon/internal/workflow"
)

func (s *Server) handleRunWorkflow(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		AgentID string         `json:"agent_id"`
		Input   map[string]any `json:"input"`
	}
	if err := readJSON(r, &req); err != nil {
		return ErrBadRequest
	}
	if req.AgentID == "" {
		return ErrBadRequest
	}

	// Add timeout to prevent indefinite execution
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	agent, err := s.db.GetAgent(ctx, req.AgentID)
	if err != nil {
		return ErrNotFound
	}

	// H-4 fix: load the agent's guardrail config and wire it (plus
	// the context manager) into the engine. The previous
	// implementation never called SetGuardrails /
	// SetContextManager, so a workflow that declared
	// RestrictedTerms or ContentPolicy was effectively running
	// without any policy enforcement. The agent execute path
	// already does this (see handlers_agent_execute.go); the
	// workflow path must do the same so the advertised feature
	// actually runs.
	var agentGuardrailCfg *models.AgentGuardrailConfig
	if agent.GuardrailConfigID != "" {
		if cfg, gerr := s.db.GetAgentGuardrailConfig(ctx, agent.GuardrailConfigID); gerr == nil {
			agentGuardrailCfg = cfg
		}
	} else if cfg, gerr := s.db.GetAgentGuardrailConfigByAgent(ctx, agent.ID); gerr == nil {
		agentGuardrailCfg = cfg
	}

	registry := workflow.DefaultRegistry()
	engine := workflow.NewEngine(registry)
	if s.guardrailManager != nil && agentGuardrailCfg != nil {
		engine.SetGuardrails(s.guardrailManager, agentGuardrailCfg)
	}
	if s.contextManager != nil {
		engine.SetContextManager(s.contextManager)
	}

	result, err := engine.Execute(ctx, agent, req.Input)
	if err != nil {
		return err
	}

	// Convert to models.Workflow for persistence
	wf := &models.Workflow{
		ID:          result.WorkflowID,
		AgentID:     agent.ID,
		Status:      models.WorkflowStatus(result.Status),
		Input:       req.Input,
		Output:      result.Outputs,
		Error:       result.Error,
		StartedAt:   result.StartedAt,
		CompletedAt: &result.FinishedAt,
		CreatedAt:   result.StartedAt,
	}

	if err := s.db.SaveWorkflow(ctx, wf); err != nil {
		s.logger.Error("failed to save workflow", "err", err, "workflow_id", wf.ID)
		return err
	}

	// Persist individual step states. M-20 fix: iterate in
	// topological order rather than the (map-iteration-random)
	// result.Steps map. The previous code wrote step rows in
	// non-deterministic order, which made the workflow_steps table
	// hard to read and to join against the parent workflow.
	for _, level := range workflow.Levels(agent.Steps) {
		for _, stepID := range level {
			sr, ok := result.Steps[stepID]
			if !ok {
				continue
			}
			startedAt := result.StartedAt
			finishedAt := result.FinishedAt
			step := &models.WorkflowStep{
				ID:         generateID(),
				WorkflowID: wf.ID,
				StepID:     stepID,
				Status:     string(sr.Status),
				Output:     sr.Output,
				Error:      sr.Error,
				ToolCalls:  sr.ToolCalls,
				LatencyMs:  sr.LatencyMs,
				StartedAt:  &startedAt,
				FinishedAt: &finishedAt,
			}
			if err := s.db.SaveWorkflowStep(ctx, step); err != nil {
				s.logger.Error("failed to save workflow step", "err", err, "step_id", stepID)
			}
		}
	}

	s.audit(ctx, "workflow_run", "agent:"+agent.ID, map[string]any{
		"workflow_id": wf.ID,
		"status":      string(wf.Status),
		"steps":       len(result.Steps),
	})

	writeJSON(w, http.StatusOK, wf)
	return nil
}

func (s *Server) handleGetWorkflow(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	wf, err := s.db.GetWorkflow(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}
	writeJSON(w, http.StatusOK, wf)
	return nil
}

func (s *Server) handleListWorkflows(w http.ResponseWriter, r *http.Request) error {
	filter := models.WorkflowFilter{
		AgentID: r.URL.Query().Get("agent_id"),
		Status:  r.URL.Query().Get("status"),
		Limit:   50,
		Offset:  0,
	}

	// Parse limit parameter
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			filter.Limit = n
		}
	}

	// Parse offset parameter
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			filter.Offset = n
		}
	}

	workflows, err := s.db.ListWorkflows(r.Context(), filter)
	if err != nil {
		return err
	}
	if workflows == nil {
		workflows = []*models.Workflow{}
	}
	writeJSON(w, http.StatusOK, workflows)
	return nil
}

func (s *Server) handleGetWorkflowSteps(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	_, err := s.db.GetWorkflow(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	steps, err := s.db.GetWorkflowSteps(r.Context(), id)
	if err != nil {
		return err
	}
	if steps == nil {
		steps = []*models.WorkflowStep{}
	}
	writeJSON(w, http.StatusOK, steps)
	return nil
}

func (s *Server) handleCancelWorkflow(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	wf, err := s.db.GetWorkflow(r.Context(), id)
	if err != nil {
		return ErrNotFound
	}

	if wf.Status != models.WorkflowPending && wf.Status != models.WorkflowRunning {
		return ErrConflict
	}

	now := time.Now()
	wf.Status = models.WorkflowCancelled
	wf.CompletedAt = &now

	if err := s.db.SaveWorkflow(r.Context(), wf); err != nil {
		return err
	}

	s.audit(r.Context(), "workflow_cancel", "agent:"+wf.AgentID, map[string]any{
		"workflow_id": wf.ID,
	})

	writeJSON(w, http.StatusOK, wf)
	return nil
}
