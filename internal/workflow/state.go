package workflow

import (
	"fmt"
	"time"

	"github.com/sachn-cs/promptsheon/internal/models"
)

// Workflow is the persisted representation of a running/completed workflow.
type Workflow struct {
	ID          string         `json:"id"`
	AgentID     string         `json:"agent_id"`
	Status      Status         `json:"status"`
	Input       map[string]any `json:"input"`
	Output      map[string]any `json:"output"`
	Steps       []*StepState   `json:"steps"`
	Error       string         `json:"error,omitempty"`
	StartedAt   time.Time      `json:"started_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// StepState tracks the state of an individual step.
type StepState struct {
	WorkflowID string            `json:"workflow_id"`
	StepID     string            `json:"step_id"`
	Status     Status            `json:"status"`
	Input      map[string]any    `json:"input"`
	Output     map[string]any    `json:"output"`
	Error      string            `json:"error,omitempty"`
	ToolCalls  []models.ToolCall `json:"tool_calls"`
	LatencyMs  int64             `json:"latency_ms"`
	StartedAt  *time.Time        `json:"started_at,omitempty"`
	FinishedAt *time.Time        `json:"finished_at,omitempty"`
}

// FromResult converts a WorkflowResult into a persisted Workflow.
func FromResult(result *WorkflowResult, agentID string, input map[string]any) *Workflow {
	w := &Workflow{
		ID:          result.WorkflowID,
		AgentID:     agentID,
		Status:      result.Status,
		Input:       input,
		Output:      result.Outputs,
		Error:       result.Error,
		StartedAt:   result.StartedAt,
		CompletedAt: &result.FinishedAt,
		CreatedAt:   result.StartedAt,
	}

	for stepID, sr := range result.Steps {
		step := &StepState{
			WorkflowID: result.WorkflowID,
			StepID:     stepID,
			Status:     sr.Status,
			Output:     sr.Output,
			Error:      sr.Error,
			ToolCalls:  sr.ToolCalls,
			LatencyMs:  sr.LatencyMs,
		}
		w.Steps = append(w.Steps, step)
	}

	return w
}

// StepByID returns the step state for the given step ID, or nil.
func (w *Workflow) StepByID(id string) *StepState {
	for _, s := range w.Steps {
		if s.StepID == id {
			return s
		}
	}
	return nil
}

// Summary returns a human-readable summary.
func (w *Workflow) Summary() string {
	return fmt.Sprintf("Workflow %s: %s (%d steps)", w.ID, w.Status, len(w.Steps))
}
