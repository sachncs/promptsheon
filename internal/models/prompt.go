// Package models defines the domain types used throughout Promptsheon.
// These types represent the core data structures for prompts, agents,
// evaluations, and other system entities.
package models

import "time"

// PromptStatus represents the lifecycle state of a prompt.
type PromptStatus string

const (
	StatusDraft    PromptStatus = "draft"
	StatusApproved PromptStatus = "approved"
	StatusDeployed PromptStatus = "deployed"
	StatusArchived PromptStatus = "archived"
)

// ValidStatusTransitions defines allowed state transitions.
var ValidStatusTransitions = map[PromptStatus][]PromptStatus{
	StatusDraft:    {StatusApproved, StatusArchived},
	StatusApproved: {StatusDeployed, StatusArchived},
	StatusDeployed: {StatusArchived},
}

// CanTransitionTo checks if a transition from current status to target is valid.
func (s PromptStatus) CanTransitionTo(target PromptStatus) bool {
	allowed, ok := ValidStatusTransitions[s]
	if !ok {
		return false
	}
	for _, a := range allowed {
		if a == target {
			return true
		}
	}
	return false
}

// Variable defines a template variable that can be substituted in a prompt.
type Variable struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "string", "number", "bool"
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description"`
}

// ProviderBinding specifies which LLM provider and model to use for execution.
type ProviderBinding struct {
	Provider  string `json:"provider"`              // e.g. "openai", "azure", "anthropic"
	Model     string `json:"model"`                 // e.g. "gpt-4", "claude-3-opus"
	APIKeyRef string `json:"api_key_ref,omitempty"` // vault key ID for API key lookup
}

// GenerationConfig holds per-prompt generation parameters.
type GenerationConfig struct {
	Temperature float64  `json:"temperature,omitempty"` // 0.0-2.0
	TopP        float64  `json:"top_p,omitempty"`       // 0.0-1.0
	MaxTokens   int      `json:"max_tokens,omitempty"`  // max output tokens
	Stop        []string `json:"stop,omitempty"`        // stop sequences
}

// Prompt represents a versioned, named prompt asset.
type Prompt struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Content      string            `json:"content"`
	SystemPrompt string            `json:"system_prompt,omitempty"` // default system prompt for all executions
	Variables    []Variable        `json:"variables"`
	Tags         []string          `json:"tags"`
	ModelHint    string            `json:"model_hint"`
	Binding      *ProviderBinding  `json:"binding,omitempty"`    // per-prompt provider resolution
	Generation   *GenerationConfig `json:"generation,omitempty"` // per-prompt generation parameters
	Version      int               `json:"version"`
	Status       PromptStatus      `json:"status"`
	Environment  string            `json:"environment"` // "dev", "staging", "prod"
	CASHash      string            `json:"cas_hash"`
	CreatedBy    string            `json:"created_by"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	Metadata     map[string]string `json:"metadata"`
}

// PromptFilter defines criteria for listing prompts.
type PromptFilter struct {
	Status      []PromptStatus
	Tags        []string
	Search      string
	Environment string
	Limit       int
	Offset      int
}
