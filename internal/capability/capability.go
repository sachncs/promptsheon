// Package capability defines the core domain model for Promptsheon.
//
// The central insight: the Capability is the root object. Everything else
// (Prompt, ModelPolicy, Guardrails, etc.) either defines, executes, observes,
// or improves a capability. A capability expresses one business outcome and
// never contains implementation details — it only has identity.
//
// Every other artifact expresses how the system currently achieves that
// outcome, and every other artifact is replaceable.
package capability

import "time"

// Workspace is the enterprise boundary. It owns projects and provides
// organization-wide policies, billing, secrets, and user management.
type Workspace struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Organization string    `json:"organization,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Project is a logical grouping of capabilities.
// Examples: Customer Support, Legal, Finance, Marketing, Internal Copilot.
type Project struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// State represents the lifecycle state of a capability.
type State string

const (
	// StateDraft is a draft state.
	StateDraft      State = "draft"
	// StateActive is an active state.
	StateActive     State = "active"
	// StateDeprecated is a deprecated state.
	StateDeprecated State = "deprecated"
	// StateArchived is an archived state.
	StateArchived   State = "archived"
)

// Capability represents one business outcome.
//
// A capability NEVER contains implementation. It only has identity.
// The business thinks in terms of capabilities ("Review a contract"),
// while Promptsheon is free to evolve the implementation behind that
// capability based on evidence from evaluations and production telemetry.
type Capability struct {
	ID               string          `json:"id"`
	ProjectID        string          `json:"project_id"`
	Name             string          `json:"name"`
	Description      string          `json:"description,omitempty"`
	Owner            string          `json:"owner,omitempty"`
	Tags             []string        `json:"tags,omitempty"`
	State            State `json:"state"`
	CurrentVersionID string          `json:"current_version_id,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}
