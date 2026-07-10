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
	StateDraft State = "draft"
	// StateActive is an active state.
	StateActive State = "active"
	// StateDeprecated is a deprecated state.
	StateDeprecated State = "deprecated"
	// StateArchived is an archived state.
	StateArchived State = "archived"
)

// Capability represents one business outcome.
//
// A capability NEVER contains implementation. It only has identity.
// The business thinks in terms of capabilities ("Review a contract"),
// while Promptsheon is free to evolve the implementation behind that
// capability based on evidence from evaluations and production telemetry.
//
// Per M0.8 / ADR-0011-follow-on, State and CurrentVersionID are
// DERIVED from Release state, not stored. A Capability moves through
// StateDraft when no Release exists for any Environment; StateActive
// when at least one Release is Active; StateDeprecated when all
// Environments are Superseded. Use DeriveState to compute.
type Capability struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Owner       string    `json:"owner,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DeriveState computes the Capability's lifecycle state from a set of
// Releases. The empty slice (no Releases) yields StateDraft; a
// mix with at least one Active Release yields StateActive; all
// Releases Superseded yields StateDeprecated.
func (c Capability) DeriveState(releases []ReleaseProbe) State {
	active := 0
	superseded := 0
	for _, r := range releases {
		switch r.Status {
		case ReleaseStatusActive:
			active++
		case ReleaseStatusSuperseded, ReleaseStatusRolledBack:
			superseded++
		}
	}
	switch {
	case active == 0 && superseded == 0:
		return StateDraft
	case active > 0:
		return StateActive
	default:
		return StateDeprecated
	}
}

// ReleaseProbe is the minimal shape DeriveState needs; the type
// lives here (not in the release package) so the capability package
// stays free of an import cycle. Callers construct values from
// release.Release values.
type ReleaseProbe struct {
	Status ReleaseStatusValue
}

// ReleaseStatusValue is the small closed set of states the
// Capability cares about. Anything not in this set is treated as
// StateDraft.
type ReleaseStatusValue string

const (
	ReleaseStatusActive     ReleaseStatusValue = "active"
	ReleaseStatusSuperseded ReleaseStatusValue = "superseded"
	ReleaseStatusRolledBack ReleaseStatusValue = "rolled_back"
)
