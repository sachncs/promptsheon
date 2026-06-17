package models

import "time"

// ToolType identifies the kind of tool a step can invoke.
type ToolType string

const (
	ToolHTTP     ToolType = "http"
	ToolShell    ToolType = "shell"
	ToolJSON     ToolType = "json_transform"
	ToolPrompt   ToolType = "prompt_call"
)

// ToolRef references a tool configuration for an agent step.
type ToolRef struct {
	Name   string         `json:"name"`
	Type   ToolType       `json:"type"`
	Config map[string]any `json:"config"`
}

// ToolCall records an actual tool invocation during execution.
type ToolCall struct {
	Tool      string         `json:"tool"`
	Input     map[string]any `json:"input"`
	Output    map[string]any `json:"output,omitempty"`
	Error     string         `json:"error,omitempty"`
	LatencyMs int64          `json:"latency_ms"`
}

// Condition defines a branching condition for a workflow step.
type Condition struct {
	Field    string `json:"field"`    // output key or variable to check
	Operator string `json:"operator"` // "eq", "neq", "contains", "gt", "lt", "exists"
	Value    string `json:"value"`    // value to compare against
}

// AgentStep defines a single unit of work within an agent workflow.
type AgentStep struct {
	ID        string      `json:"id"`
	PromptID  string      `json:"prompt_id"`
	PromptHash string    `json:"prompt_hash"` // CAS hash for immutability
	DependsOn []string    `json:"depends_on"`
	ToolCalls []ToolCall  `json:"tool_calls"`
	OutputKey string      `json:"output_key"` // key for passing output to next step
	Condition *Condition  `json:"condition,omitempty"` // optional branching condition
}

// Agent represents a multi-step agent workflow.
type Agent struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Steps       []AgentStep  `json:"steps"`
	Tools       []ToolRef    `json:"tools"`
	Status      PromptStatus `json:"status"`
	IsTemplate  bool         `json:"is_template"`  // template agents can be forked
	ParentID    string       `json:"parent_id"`    // original agent ID if forked
	CASHash     string       `json:"cas_hash"`
	CreatedBy   string       `json:"created_by"`
	Tags        []string     `json:"tags"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}
