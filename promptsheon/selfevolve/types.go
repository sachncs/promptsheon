// Package selfevolve runs the closed-loop self-evolution
// orchestrator. The evolver watches the latest EvalRun for
// a Capability's active Release; when the score drops below
// the configured threshold it:
//
//  1. reads the failing cases from that EvalRun
//  2. asks a revision LLM for a new prompt
//  3. validates the candidate against the same dataset
//  4. on success, writes the new prompt to CAS, creates a
//     new Version + Release, and SelfActivate-s it in the
//     configured env (typically dev)
//
// Safety rails: max revisions per cycle, cooldown between
// cycles, a dedicated SelfApprovePolicy that only the
// evolver's activator uses, every state change in the
// audit chain.
package selfevolve

import (
	"github.com/sachncs/promptsheon/promptsheon/store"
	"github.com/sachncs/promptsheon/promptsheon/capability"
	"github.com/sachncs/promptsheon/promptsheon/harness"
	"context"
	"time"

)

// Audit action keys. The evolver writes one of these per
// state transition; operators tail the audit chain to
// follow a cycle in real time.
const (
	AuditDetect   = "self_evolve.detect"
	AuditRevise   = "self_evolve.revise"
	AuditValidate = "self_evolve.validate"
	AuditPromote  = "self_evolve.promote"
	AuditReject   = "self_evolve.reject"
	AuditSkip     = "self_evolve.skip"
)

// Cycle status strings. Written to
// store.SelfEvolveState.LastStatus; surfaced in the audit
// row details.
const (
	StatusIdle       = "idle"
	StatusDetected   = "detected"
	StatusRevising   = "revising"
	StatusValidating = "validating"
	StatusPromoted   = "promoted"
	StatusRejected   = "rejected"
)

// Repository is the persistent surface the evolver reads
// and writes. It composes the harness repository with
// capability-level reads/writes and a handful of evolver-
// specific methods. A single interface keeps main.go
// from wiring ten concrete fields into Evolver and the
// test suite from building ten separate fakes.
type Repository interface {
	CapabilityRepository
	harness.Repository

	LoadSelfEvolveState(ctx context.Context, capabilityID, targetEnv string) (*store.SelfEvolveState, error)
	SaveSelfEvolveState(ctx context.Context, st *store.SelfEvolveState) error

	// ActiveReleaseID returns the active release id for
	// the capability in env, or "" if none.
	ActiveReleaseID(ctx context.Context, capabilityID, env string) (string, error)

	// GetRelease returns the release by id (used to copy
	// model_policy / runtime_policy hashes from the active
	// release to the candidate version).
	GetRelease(ctx context.Context, id string) (*ReleaseRecord, error)

	// LastEvalRun returns the most recent EvalRun for the
	// release, or (nil, nil) if none. The evolver uses
	// this to read the latest score without scanning the
	// table.
	LastEvalRun(ctx context.Context, releaseID string) (*harness.EvalRun, error)

	// UpdateReleaseStatus marks a release as superseded or
	// rolled back; used when the evolver aborts a cycle.
	UpdateReleaseStatus(ctx context.Context, releaseID string, status string) error

	// CreateRelease creates a Pending release. The evolver
	// uses this to spin up the candidate release in the
	// target env; SelfActivate flips it to Active.
	CreateRelease(ctx context.Context, r ReleaseRecord) error
}

// CapabilityRepository is the capability.Repository surface
// the evolver needs. Kept as a named interface so the
// evolver's test fake can implement just this subset.
type CapabilityRepository interface {
	GetCapability(ctx context.Context, id string) (*capability.Capability, error)
	GetVersion(ctx context.Context, id string) (*capability.Version, error)
	GetVersionByNumber(ctx context.Context, capabilityID string, version int) (*capability.Version, error)
	CreateVersion(ctx context.Context, v *capability.Version) error
	UpdateSelfEvolveConfig(ctx context.Context, capabilityID string, cfg capability.SelfEvolveConfig) error
}

// ReleaseRecord is the input to Repository.CreateRelease.
// It mirrors release.Release without dragging the
// release.Service surface into the evolver's interface.
// The evolver fills it from the active release's manifest
// plus the new prompt hash.
type ReleaseRecord struct {
	ID                string
	CapabilityID      string
	CapabilityVersion int
	Manifest          capability.Manifest
	Environment       string
	Status            string
	CreatedBy         string
	CreatedAt         time.Time
}

// PromptLoader reads and writes prompt blobs in CAS. The
// evolver uses it to fetch the active prompt and to
// persist the candidate prompt.
type PromptLoader interface {
	LoadPrompt(ctx context.Context, hash string) ([]byte, error)
	WritePrompt(ctx context.Context, text string) (hash string, err error)
}

// ReleaseActivator activates a release. The evolver wires
// a SelfApprovePolicy-wrapped activator so the
// maker-checker does not block the auto-promote.
type ReleaseActivator interface {
	SelfActivate(ctx context.Context, releaseID string) error
}

// RevisionLLM proposes a revised prompt. The evolver
// supplies the current prompt + failing cases; the LLM
// returns the new prompt text.
type RevisionLLM interface {
	Revise(ctx context.Context, req ReviseRequest) (*ReviseResponse, error)
}

// ReviseRequest is the input to RevisionLLM.Revise.
type ReviseRequest struct {
	CapabilityID    string
	CurrentPrompt   string
	CurrentHash     string
	ModelPolicyHash string
	FailingCases    []FailingCase
}

// FailingCase is one (input, expected, actual) tuple where
// the current prompt produced the wrong answer. The
// revision LLM uses this to drive its proposal.
type FailingCase struct {
	Seq      int
	Inputs   map[string]any
	Expected string
	Actual   string
	Reason   string
}

// ReviseResponse is the output of RevisionLLM.Revise.
type ReviseResponse struct {
	NewPrompt string
	Rationale string
	// Hash is set by the evolver after writing the new
	// prompt to CAS; the LLM does not return it.
	Hash string
}

// Result is the return value of Evolver.RunOnce. The
// main-loop caller uses it to log and feed metrics.
type Result struct {
	CapabilityID string
	Started      bool
	Promoted     bool
	Skipped      bool
	RejectReason string
	Score        float64
	Revisions    int
	DurationMS   int64
}

// DefaultRevisionLLMSystem is the system prompt the
// evolver hands to the revision LLM. The LLM is
// instructed to return only the new prompt text.
const DefaultRevisionLLMSystem = `You are a prompt engineer. You will be given a current system prompt and a list of (input, expected_output, actual_output) cases where the model produced the wrong answer. Return a revised system prompt that would make the model produce the expected output.

Rules:
- Output ONLY the revised system prompt text. No preamble, no commentary, no markdown, no code fences.
- Keep the original style and constraints. Only change what is necessary to fix the failing cases.
- Do not add new examples or instructions that are not implied by the failing cases.
- Do not mention the failing cases in the prompt itself.
- Length should be similar to the original unless a structural change is required.`

// LLMInvokeRequest is the wire format the evolver sends
// to the LLM. The revision strategy uses
// DefaultRevisionLLMSystem as System; the validator uses
// the candidate prompt as System.
type LLMInvokeRequest struct {
	System string
	User   string
}

// LLMInvokeFn is the function shape the evolver uses to
// call any LLM. Production wires it to the daemon's
// llm.Registry; tests pass a fake.
type LLMInvokeFn func(ctx context.Context, req LLMInvokeRequest) (string, error)

// DefaultCycleTimeout caps a single RunOnce. The evolver
// does not currently use this; the persisted state row
// keeps partial cycle state so a future invocation can
// resume.
const DefaultCycleTimeout = 5 * time.Minute
