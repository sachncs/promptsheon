package models

import "time"

// ContextType identifies the kind of context.
type ContextType string

const (
	ContextSystemPrompt ContextType = "system_prompt"
	ContextConversation ContextType = "conversation"
	ContextComposite    ContextType = "composite"
)

// TruncationStrategy defines how context is trimmed when over budget.
type TruncationStrategy string

const (
	TruncationSlidingWindow TruncationStrategy = "sliding_window"
	TruncationDropOldest    TruncationStrategy = "drop_oldest"
)

// ContextMessage represents a single message in a conversation context.
type ContextMessage struct {
	ID         string    `json:"id"`
	Role       string    `json:"role"` // "system", "user", "assistant"
	Content    string    `json:"content"`
	TokenCount int       `json:"token_count"`
	CreatedAt  time.Time `json:"created_at"`
}

// Context represents a reusable context asset that can be attached to agent steps.
// It manages system prompts, conversation history, and token budgets.
type Context struct {
	ID                 string             `json:"id"`
	Name               string             `json:"name"`
	Description        string             `json:"description"`
	Type               ContextType        `json:"type"`
	SystemPrompt       string             `json:"system_prompt"`
	Messages           []ContextMessage   `json:"messages"`
	TokenBudget        int                `json:"token_budget"`
	TokenCount         int                `json:"token_count"`
	TruncationStrategy TruncationStrategy `json:"truncation_strategy"`
	AgentID            string             `json:"agent_id,omitempty"`
	Version            int                `json:"version"`
	Status             PromptStatus       `json:"status"`
	Metadata           map[string]string  `json:"metadata"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
}

// ContextFilter defines criteria for listing contexts.
type ContextFilter struct {
	AgentID string
	Type    ContextType
	Search  string
	Status  []PromptStatus
	Limit   int
	Offset  int
}
