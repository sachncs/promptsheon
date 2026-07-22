// Package workflow provides workflow orchestration for capability execution.
package workflow

import (
	"time"

	"github.com/sachncs/promptsheon/internal/metrics"
	"github.com/sachncs/promptsheon/internal/trace"
)

// Status represents a workflow execution status.
type Status string

const (
	// StatusPending is a pending workflow status.
	StatusPending Status = "pending"
	// StatusRunning is a running workflow status.
	StatusRunning Status = "running"
	// StatusCompleted is a completed workflow status.
	StatusCompleted Status = "completed"
	// StatusFailed is a failed workflow status.
	StatusFailed Status = "failed"
	// StatusCancelled is a cancelled workflow status.
	StatusCancelled Status = "cancelled"
	// StatusSkipped is a skipped workflow status.
	StatusSkipped Status = "skipped"
)

// StepResult contains the outcome of a single workflow step.
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

// Result holds the complete output of a workflow execution.
type Result struct {
	WorkflowID string                 `json:"workflow_id"`
	Status     Status                 `json:"status"`
	Steps      map[string]*StepResult `json:"steps"`
	Outputs    map[string]any         `json:"outputs"`
	StartedAt  time.Time              `json:"started_at"`
	FinishedAt time.Time              `json:"finished_at"`
	Error      string                 `json:"error,omitempty"`
}

// Engine orchestrates workflow execution.
type Engine struct {
	toolRegistry *Registry
	guardrailMgr any
	agentConfig  any
	contextMgr   any

	// OBS-5b: optional observability wiring. When non-nil, every
	// Run records workflow_total / active counters, duration
	// histogram, and an OTel span.
	llmCollector *metrics.Collector
	tracer       trace.Tracer
}

// NewEngine creates a new Engine.
func NewEngine(registry *Registry) *Engine {
	return &Engine{toolRegistry: registry}
}

// WithObservability wires the metrics collector + OTel tracer
// used by the WorkflowMiddleware wrapper. OBS-5b.
func (e *Engine) WithObservability(c *metrics.Collector, t trace.Tracer) *Engine {
	e.llmCollector = c
	e.tracer = t
	return e
}

// SetGuardrails configures guardrail manager and agent config.
func (e *Engine) SetGuardrails(mgr, cfg any) {
	e.guardrailMgr = mgr
	e.agentConfig = cfg
}

// SetContextManager configures the context manager.
func (e *Engine) SetContextManager(mgr any) {
	e.contextMgr = mgr
}
