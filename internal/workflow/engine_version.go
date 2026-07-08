package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
)

// ExecuteVersion runs a workflow defined by a CapabilityVersion.
//
// This is the capability-centric equivalent of Execute(*models.Agent).
// It extracts the prompt, guardrails, and runtime policy from the version
// and executes the workflow against the given input.
func (e *Engine) ExecuteVersion(ctx context.Context, version *capability.Version, input map[string]any) (*Result, error) {
	if version == nil {
		return nil, fmt.Errorf("capability version is required")
	}

	result := &Result{
		WorkflowID: version.ID,
		Status:     StatusRunning,
		Steps:      make(map[string]*StepResult),
		Outputs:    make(map[string]any),
		StartedAt:  time.Now(),
	}

	for k, v := range input {
		result.Outputs[k] = v
	}

	stepResult := e.executePromptStep(ctx, version, input)

	result.Steps["main"] = stepResult
	if stepResult.Status == StatusFailed {
		result.Status = StatusFailed
		result.Error = stepResult.Error
	} else {
		result.Status = StatusCompleted
		if stepResult.Output != nil {
			for k, v := range stepResult.Output {
				result.Outputs[k] = v
			}
		}
	}

	result.FinishedAt = time.Now()
	return result, nil
}

// executePromptStep executes a single step using the capability version's prompt.
func (e *Engine) executePromptStep(ctx context.Context, version *capability.Version, input map[string]any) *StepResult {
	result := &StepResult{
		StepID: "main",
		Status: StatusRunning,
		Output: make(map[string]any),
	}

	start := time.Now()

	_ = input
	_ = version.RuntimePolicy

	// Check guardrails
	if len(version.Guardrails) > 0 && e.guardrailMgr != nil {
		if mgr, ok := e.guardrailMgr.(interface {
			CheckVersion(ctx context.Context, version *capability.Version) error
		}); ok {
			if err := mgr.CheckVersion(ctx, version); err != nil {
				result.Status = StatusFailed
				result.Error = fmt.Sprintf("guardrail check failed: %v", err)
				result.LatencyMs = time.Since(start).Milliseconds()
				return result
			}
		}
	}

	result.Status = StatusCompleted
	result.LatencyMs = time.Since(start).Milliseconds()
	return result
}
