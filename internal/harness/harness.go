// Package harness defines the harness-engineering domain types:
//
//   - Dataset:     a collection of test cases (input + expected output)
//                  attached to a Capability. The ground truth for evals.
//   - Precondition: a named command hook attached to a Capability.
//                  The harness runner executes them on Activate; a
//                  failing precondition blocks the Release.
//   - EvalRun:     a recorded eval invocation against a Release.
//                  Per-case outcomes + aggregate score.
//   - EvalResult:  per-case outcome within an EvalRun.
//
// These types are value-immutable: mutations produce a new value
// (the same convention as Release and Approval). Storage handles
// persisting them; the harness service runs them.
package harness

import (
	"encoding/json"
	"errors"
	"time"
)

// Scorer is the registered name of a built-in or user-supplied
// scoring strategy. Valid values today are ScorerExactMatch,
// ScorerContains, ScorerRegex, and ScorerJSONSchema.
type Scorer string

const (
	ScorerExactMatch Scorer = "exact_match"
	ScorerContains   Scorer = "contains"
	ScorerRegex      Scorer = "regex"
	ScorerJSONSchema Scorer = "json_schema"
)

// ValidScorers reports whether s is a recognised scorer name.
func ValidScorers(s Scorer) bool {
	switch s {
	case ScorerExactMatch, ScorerContains, ScorerRegex, ScorerJSONSchema:
		return true
	}
	return false
}

// RunStatus is the lifecycle state of an EvalRun.
type RunStatus string

const (
	RunRunning RunStatus = "running"
	RunPassed  RunStatus = "passed"
	RunFailed  RunStatus = "failed"
	RunError   RunStatus = "error"
)

// ErrPreconditionFailed is returned by Activate when one or more
// preconditions for the Release's Capability failed. Failures
// carries the per-hook name + output for the handler to surface.
var ErrPreconditionFailed = errors.New("harness: precondition failed")

// Failure describes one failing precondition.
type Failure struct {
	Name   string `json:"name"`
	Output string `json:"output"`
}

// Dataset is a named collection of test cases.
type Dataset struct {
	ID           string    `json:"id"`
	CapabilityID string    `json:"capability_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Cases        []DatasetCase `json:"cases,omitempty"`
}

// DatasetCase is one row of a Dataset: input + expected output.
type DatasetCase struct {
	ID          string          `json:"id"`
	DatasetID   string          `json:"dataset_id"`
	Seq         int             `json:"seq"`
	Inputs      json.RawMessage `json:"inputs"`
	Expected    json.RawMessage `json:"expected"`
	Description string          `json:"description,omitempty"`
}

// Precondition is a named command hook attached to a Capability.
// The daemon runs Enabled preconditions when a Release is activated.
type Precondition struct {
	ID           string    `json:"id"`
	CapabilityID string    `json:"capability_id"`
	Name         string    `json:"name"`
	Command      string    `json:"command"`
	TimeoutSec   int       `json:"timeout_sec"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
}

// Validate enforces the precondition invariants.
func (p Precondition) Validate() error {
	if p.CapabilityID == "" {
		return errors.New("harness: precondition capability_id is required")
	}
	if p.Name == "" {
		return errors.New("harness: precondition name is required")
	}
	if p.Command == "" {
		return errors.New("harness: precondition command is required")
	}
	if p.TimeoutSec <= 0 {
		return errors.New("harness: precondition timeout_sec must be positive")
	}
	return nil
}

// EvalRun is a recorded invocation of an eval against a Release.
// Run-time fields (Score, Passed, Failed, Total, Status,
// FinishedAt) are populated as the run progresses.
type EvalRun struct {
	ID          string     `json:"id"`
	ReleaseID   string     `json:"release_id"`
	DatasetID   string     `json:"dataset_id"`
	Scorer      Scorer     `json:"scorer"`
	Score       float64    `json:"score"`
	Passed      int        `json:"passed"`
	Failed      int        `json:"failed"`
	Total        int        `json:"total"`
	Status      RunStatus  `json:"status"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at,omitempty"`
}

// EvalResult is the per-case outcome within an EvalRun.
type EvalResult struct {
	ID        string          `json:"id"`
	RunID     string          `json:"run_id"`
	CaseID    string          `json:"case_id"`
	Seq       int             `json:"seq"`
	Passed    bool            `json:"passed"`
	Actual    json.RawMessage `json:"actual"`
	Error     string          `json:"error,omitempty"`
	LatencyMs  int64           `json:"latency_ms"`
}
