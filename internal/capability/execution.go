package capability

import "time"

// Execution records a single invocation of a capability version.
//
// This is runtime, not configuration. Each execution captures everything
// about one call — the inputs, outputs, which model was used, latency,
// cost, token usage, and any errors that occurred.
// Millions of these exist per capability.
type Execution struct {
	ID                  string         `json:"id"`
	CapabilityVersionID string         `json:"capability_version_id"`
	Timestamp           time.Time      `json:"timestamp"`
	Inputs              map[string]any `json:"inputs,omitempty"`
	Outputs             map[string]any `json:"outputs,omitempty"`
	Model               string         `json:"model"`
	Provider            string         `json:"provider"`
	LatencyMs           int64          `json:"latency_ms"`
	CostUSD             float64        `json:"cost_usd"`
	PromptTokens        int            `json:"prompt_tokens"`
	CompletionTokens    int            `json:"completion_tokens"`
	TotalTokens         int            `json:"total_tokens"`
	Error               string         `json:"error,omitempty"`
	TraceID             string         `json:"trace_id,omitempty"`
	Environment         string         `json:"environment,omitempty"`
}
