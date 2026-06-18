package models

import "time"

// AgentGuardrailConfig defines guardrail settings for a specific agent.
type AgentGuardrailConfig struct {
	ID               string    `json:"id"`
	AgentID          string    `json:"agent_id"`
	Name             string    `json:"name"`
	Enabled          bool      `json:"enabled"`
	MaxCostPerRun    float64   `json:"max_cost_per_run,omitempty"`
	MaxLatencyMs     int64     `json:"max_latency_ms,omitempty"`
	MaxTokensPerStep int       `json:"max_tokens_per_step,omitempty"`
	ContentPolicy    []string  `json:"content_policy,omitempty"`   // e.g. ["no_pii", "no_harmful"]
	RestrictedTerms  []string  `json:"restricted_terms,omitempty"` // terms that trigger violations
	StopOnViolation  bool      `json:"stop_on_violation"`          // abort workflow on critical violation
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
