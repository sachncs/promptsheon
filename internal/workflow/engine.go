package workflow

import (
	"time"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
	StatusSkipped   Status = "skipped"
)

type StepResult struct {
	StepID     string         `json:"step_id"`
	Status     Status         `json:"status"`
	Output     map[string]any `json:"output,omitempty"`
	Error      string         `json:"error,omitempty"`
	LatencyMs  int64          `json:"latency_ms"`
	CostUSD    float64        `json:"cost_usd,omitempty"`
	TokensUsed int            `json:"tokens_used,omitempty"`
	Model      string         `json:"model,omitempty"`
	Provider   string         `json:"provider,omitempty"`
}

type WorkflowResult struct {
	WorkflowID string                 `json:"workflow_id"`
	Status     Status                 `json:"status"`
	Steps      map[string]*StepResult `json:"steps"`
	Outputs    map[string]any         `json:"outputs"`
	StartedAt  time.Time              `json:"started_at"`
	FinishedAt time.Time              `json:"finished_at"`
	Error      string                 `json:"error,omitempty"`
}

type Engine struct {
	toolRegistry *Registry
	guardrailMgr any
	agentConfig  any
	contextMgr   any
}

func NewEngine(registry *Registry) *Engine {
	return &Engine{toolRegistry: registry}
}

func (e *Engine) SetGuardrails(mgr any, cfg any) {
	e.guardrailMgr = mgr
	e.agentConfig = cfg
}

func (e *Engine) SetContextManager(mgr any) {
	e.contextMgr = mgr
}
