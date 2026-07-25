// Package capability defines the core domain model for Promptsheon.
//
// The central insight: the Capability is the root object. Everything else
// (Prompt, ModelPolicy, Guardrails, etc.) either defines, executes, observes,
// or improves a capability. A capability expresses one business outcome and
// never contains implementation details — it only has identity.
//
// Every other artifact expresses how the system currently achieves that
// outcome, and every other artifact is replaceable.
//
// DEAD-1 sweep: ReleaseProbe, ReleaseStatusValue, DeriveState,
// Observation, EvaluationResult, and the 11 unused EventType
// constants were removed in this commit. DeriveState had no
// production caller; the capability state lives implicitly in
// the Release.Status field that drives every read path.
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

// Capability represents one business outcome.
//
// A capability NEVER contains implementation. It only has identity.
// The business thinks in terms of capabilities ("Review a contract"),
// while Promptsheon is free to evolve the implementation behind that
// capability based on evidence from evaluations and production telemetry.
//
// State is no longer a field on Capability; the derived state lives
// implicitly in the Releases that point at this Capability.
// Queries that need a Capability's effective state should join
// against the releases table.
type Capability struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Owner       string    `json:"owner,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Contract is the typed contract this Capability promises
	// to operators. A Capability without a Contract cannot be
	// auto-promoted by the Recommendation engine. The field is
	// optional for back-compat with v0.2.0 Capabilities that
	// predate the Contract primitive; new Capabilities SHOULD
	// attach one.
	Contract *CapabilityContract `json:"contract,omitempty"`

	// SelfEvolve is the closed-loop self-evolution config. When
	// SelfEvolve.Enabled is true and the active release's
	// EvalRun score drops below MinScore, the daemon
	// automatically revises the prompt artifact, validates the
	// candidate version against the same dataset, and promotes
	// the validated version in TargetEnv. See
	// internal/selfevolve for the orchestrator and the safety
	// rails.
	SelfEvolve SelfEvolveConfig `json:"self_evolve"`
}

// SelfEvolveConfig is the per-Capability self-evolution policy.
// All fields have safe defaults via the SQLite DEFAULT clauses on
// the matching columns; JSON marshalling always emits the struct
// (zero-value fields are explicit) so API consumers see the config.
type SelfEvolveConfig struct {
	Enabled       bool    `json:"enabled"`
	MinScore      float64 `json:"min_score"`        // promote if validated score >= this
	MaxRevisions  int     `json:"max_revisions"`   // hard cap per cycle
	CooldownSec   int     `json:"cooldown_sec"`    // minimum gap between cycles
	TargetEnv     string  `json:"target_env"`      // env to auto-promote in (typically dev)
	DatasetID     string  `json:"dataset_id"`      // dataset the candidate version is validated against
}

// IsZero reports whether the config is the unset default. Callers
// use this to skip SelfEvolve entirely without inspecting every
// field. A zero-value SelfEvolve has Enabled=false, MinScore=0.9
// (the default threshold), MaxRevisions=10, CooldownSec=900,
// TargetEnv=dev, DatasetID="".
func (c SelfEvolveConfig) IsZero() bool {
	return !c.Enabled && c.MinScore == 0 && c.MaxRevisions == 0 &&
		c.CooldownSec == 0 && c.TargetEnv == "" && c.DatasetID == ""
}
