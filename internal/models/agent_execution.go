package models

import "time"

// AgentExecution records a full orchestration run of an agent.
type AgentExecution struct {
	ID                  string               `json:"id"`
	AgentID             string               `json:"agent_id"`
	WorkflowID          string               `json:"workflow_id"`
	Status              string               `json:"status"` // "pending", "running", "completed", "failed"
	Input               map[string]any       `json:"input"`
	Output              map[string]any       `json:"output"`
	Steps               []AgentExecutionStep `json:"steps"`
	TotalCostUSD        float64              `json:"total_cost_usd"`
	TotalLatencyMs      int64                `json:"total_latency_ms"`
	TotalTokens         int                  `json:"total_tokens"`
	GuardrailViolations []string             `json:"guardrail_violations,omitempty"`
	ContextID           string               `json:"context_id,omitempty"`
	CreatedAt           time.Time            `json:"created_at"`
	CompletedAt         *time.Time           `json:"completed_at,omitempty"`
}

// AgentExecutionStep records a single step within an agent execution.
type AgentExecutionStep struct {
	StepID          string         `json:"step_id"`
	Status          string         `json:"status"`
	Output          map[string]any `json:"output,omitempty"`
	Error           string         `json:"error,omitempty"`
	ToolCalls       []ToolCall     `json:"tool_calls,omitempty"`
	LatencyMs       int64          `json:"latency_ms"`
	CostUSD         float64        `json:"cost_usd,omitempty"`
	TokensUsed      int            `json:"tokens_used,omitempty"`
	Model           string         `json:"model,omitempty"`
	Provider        string         `json:"provider,omitempty"`
}
