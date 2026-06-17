package models

import "time"

// ExecutionLog records a single prompt execution event.
type ExecutionLog struct {
	ID              string            `json:"id"`
	PromptID        string            `json:"prompt_id"`
	PromptName      string            `json:"prompt_name"`
	PromptVersion   int               `json:"prompt_version"`
	Provider        string            `json:"provider"`
	Model           string            `json:"model"`
	Status          string            `json:"status"` // "success", "error", "guardrail_blocked"
	Variables       map[string]string `json:"variables,omitempty"`
	SystemPrompt    string            `json:"system_prompt,omitempty"`
	RequestMessages int               `json:"request_messages"`
	PromptTokens    int               `json:"prompt_tokens"`
	CompletionTokens int              `json:"completion_tokens"`
	TotalTokens     int               `json:"total_tokens"`
	CostUSD         float64           `json:"cost_usd"`
	LatencyMs       int64             `json:"latency_ms"`
	TraceID         string            `json:"trace_id,omitempty"`
	Error           string            `json:"error,omitempty"`
	Violations      []string          `json:"violations,omitempty"`
	Environment     string            `json:"environment"`
	CreatedAt       time.Time         `json:"created_at"`
}

// ExecutionLogFilter defines criteria for listing execution logs.
type ExecutionLogFilter struct {
	PromptID   string
	Provider   string
	Model      string
	Status     string
	Since      *time.Time
	Until      *time.Time
	Limit      int
	Offset     int
}
