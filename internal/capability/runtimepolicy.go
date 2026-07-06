package capability

// RuntimePolicy defines execution behavior for a capability version.
//
// Unlike the prompt or model policy, runtime policy is purely about
// how the execution is carried out — retries, timeouts, streaming,
// parallelism, caching, and generation parameters.
type RuntimePolicy struct {
	Retries         int     `json:"retries,omitempty"`
	TimeoutMs       int     `json:"timeout_ms,omitempty"`
	Streaming       bool    `json:"streaming,omitempty"`
	Parallelism     int     `json:"parallelism,omitempty"`
	Caching         string  `json:"caching,omitempty"` // "enabled", "disabled", "semantic"
	Temperature     float64 `json:"temperature,omitempty"`
	MaxTokens       int     `json:"max_tokens,omitempty"`
	ReasoningBudget int     `json:"reasoning_budget,omitempty"`
}
