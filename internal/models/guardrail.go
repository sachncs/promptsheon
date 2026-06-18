package models

import "time"

// GuardrailRule represents a persisted guardrail rule.
type GuardrailRule struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Type         string         `json:"type"`
	Severity     string         `json:"severity"`
	Enabled      bool           `json:"enabled"`
	Config       map[string]any `json:"config,omitempty"`
	Environments []string       `json:"environments,omitempty"`
	PromptIDs    []string       `json:"prompt_ids,omitempty"`
	AgentIDs     []string       `json:"agent_ids,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// GuardrailViolationRecord represents a persisted guardrail violation.
type GuardrailViolationRecord struct {
	ID           string         `json:"id"`
	RuleID       string         `json:"rule_id"`
	RuleName     string         `json:"rule_name"`
	Type         string         `json:"type"`
	Severity     string         `json:"severity"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	UserID       string         `json:"user_id"`
	Message      string         `json:"message"`
	Details      map[string]any `json:"details,omitempty"`
	Resolved     bool           `json:"resolved"`
	ResolvedBy   string         `json:"resolved_by,omitempty"`
	ResolvedAt   *time.Time     `json:"resolved_at,omitempty"`
	Timestamp    time.Time      `json:"timestamp"`
}
