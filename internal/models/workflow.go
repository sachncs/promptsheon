package models

import "time"

// WorkflowStatus represents the lifecycle state of a workflow.
type WorkflowStatus string

const (
	WorkflowPending   WorkflowStatus = "pending"
	WorkflowRunning   WorkflowStatus = "running"
	WorkflowCompleted WorkflowStatus = "completed"
	WorkflowFailed    WorkflowStatus = "failed"
	WorkflowCancelled WorkflowStatus = "cancelled"
)

// Workflow tracks the execution of an agent workflow.
type Workflow struct {
	ID          string         `json:"id"`
	AgentID     string         `json:"agent_id"`
	Status      WorkflowStatus `json:"status"`
	Input       map[string]any `json:"input"`
	Output      map[string]any `json:"output"`
	Error       string         `json:"error,omitempty"`
	StartedAt   time.Time      `json:"started_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

// WorkflowStep tracks the execution of a single step within a workflow.
type WorkflowStep struct {
	ID         string         `json:"id"`
	WorkflowID string         `json:"workflow_id"`
	StepID     string         `json:"step_id"`
	Status     string         `json:"status"`
	Input      map[string]any `json:"input"`
	Output     map[string]any `json:"output"`
	Error      string         `json:"error,omitempty"`
	ToolCalls  []ToolCall     `json:"tool_calls"`
	LatencyMs  int64          `json:"latency_ms"`
	StartedAt  *time.Time     `json:"started_at,omitempty"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
}

// WorkflowFilter defines criteria for listing workflows.
type WorkflowFilter struct {
	AgentID string
	Status  string
	Limit   int
	Offset  int
}
