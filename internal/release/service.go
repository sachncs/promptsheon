// Package release.Service is the application layer that wires the
// release.Repository and approval.Repository behind a MakerChecker
// policy. Handlers stay dumb: they call Service methods and map the
// returned errors to HTTP status codes.
package release

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sachncs/promptsheon/internal/approval"
	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/harness"
)

// PolicyKind selects the quorum policy at construction time.
type PolicyKind int

const (
	PolicyMakerChecker PolicyKind = iota
	PolicyMajority
)

// Service is the application layer for Release + Approval.
//
// It owns the persistence interface, the policy, and the timing
// conventions. Callers construct it once with NewService and inject
// it into HTTP handlers.
type Service struct {
	DB        Repository
	App       approval.Repository
	HarnessDB harness.Repository
	Policy    approval.Policy
	Harness   *harness.PreconditionRunner
	Clock     func() time.Time
}

// NewService constructs a Service with the supplied policy. Callers
// that want a clock seam should set Service.Clock after construction.
func NewService(db Repository, app approval.Repository, policy approval.Policy) *Service {
	return &Service{DB: db, App: app, Policy: policy, Clock: func() time.Time { return time.Now().UTC() }}
}

// NewServiceFromKind is a convenience that maps a PolicyKind enum to
// the corresponding policy. Required is the approver threshold; for
// MajorityPolicy it is the absolute count, for MakerCheckerPolicy it
// is the required non-creator approvers.
func NewServiceFromKind(db Repository, app approval.Repository, kind PolicyKind, required int) *Service {
	switch kind {
	case PolicyMajority:
		return NewService(db, app, approval.MajorityPolicy{Required: required})
	default:
		return NewService(db, app, approval.MakerCheckerPolicy{RequiredApprovers: required})
	}
}

// WithHarness attaches a PreconditionRunner + the harness
// repository it reads preconditions from. When set, Activate runs
// the registered preconditions for the Release's Capability; a
// failing precondition returns a *harness.PreconditionError.
func (s *Service) WithHarness(h *harness.PreconditionRunner, db harness.Repository) *Service {
	s.Harness = h
	s.HarnessDB = db
	return s
}

// runner is the public surface that api.Server depends on; it lets
// the compile-time assertion below catch signature drift.
type runner interface {
	Get(ctx context.Context, id string) (*Release, error)
	ListForCapability(ctx context.Context, capabilityID string) ([]*Release, error)
	ListActiveForEnvironment(ctx context.Context, env Environment) ([]*Release, error)
	Create(ctx context.Context, capabilityID string, capabilityVersion int, manifest capability.Manifest, environment Environment, createdBy string) (*Release, error)
	Vote(ctx context.Context, releaseID string, vote approval.Vote) (*approval.Approval, error)
	Activate(ctx context.Context, releaseID string) (*Release, error)
	Rollback(ctx context.Context, releaseID string) (*Release, error)
	Approval(ctx context.Context, releaseID string) (*approval.Approval, error)
}

// Service satisfies the runner interface used by api.Server.
var _ runner = (*Service)(nil)

// Create constructs a Pending Release for the given Capability Version
// and target environment. versionID is looked up; releaseID is server-
// generated.
func (s *Service) Create(ctx context.Context, capabilityID string, capabilityVersion int, manifest capability.Manifest, environment Environment, createdBy string) (*Release, error) {
	r, err := New(capabilityID, capabilityVersion, manifest, environment, createdBy)
	if err != nil {
		return nil, err
	}
	r.ID = generateReleaseID(capabilityID)
	if err := s.DB.CreateRelease(ctx, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// Vote records a single vote on the release's Approval, persists it,
// and returns the updated Approval.
func (s *Service) Vote(ctx context.Context, releaseID string, vote approval.Vote) (*approval.Approval, error) {
	a, err := s.App.GetApproval(ctx, releaseID)
	if errors.Is(err, approval.ErrNotFound) {
		// First vote on this release.
		a = &approval.Approval{ReleaseID: releaseID, UpdatedAt: s.Clock()}
	} else if err != nil {
		return nil, err
	}
	if vote.Timestamp.IsZero() {
		vote.Timestamp = s.Clock()
	}
	updated, err := a.Record(vote)
	if err != nil {
		return nil, err
	}
	if len(a.Votes) == 0 {
		if err := s.App.CreateApproval(ctx, &updated); err != nil {
			return nil, err
		}
	} else {
		if err := s.App.UpdateApproval(ctx, &updated); err != nil {
			return nil, err
		}
	}
	return &updated, nil
}

// Activate transitions a Pending Release to Active. The policy is
// consulted: if the Approval satisfies the policy, the Release moves
// to Approved then Active in one transaction. If a prior Release is
// Active in the same Environment for the same Capability, it is
// Superseded by the new Release (also in the same transaction).
//
// Activate returns the activated Release. It is the only place where
// Status moves from Pending to Active.
//
// Atomicity: the prior-supersede and next-activate are persisted in
// a single SQLite transaction via Repository.ActivateAtomic. The
// partial unique index on (capability_id, environment) WHERE
// status='active' makes the "exactly one active release per
// (capability, env)" invariant a hard database-level constraint:
// a concurrent Activate on the same capability+env returns
// SQLITE_CONSTRAINT and the caller sees a 409.
func (s *Service) Activate(ctx context.Context, releaseID string) (*Release, error) {
	r, err := s.DB.GetRelease(ctx, releaseID)
	if err != nil {
		return nil, err
	}
	if r.Status != StatusPending {
		return nil, ErrNotPending
	}

	a, err := s.App.GetApproval(ctx, releaseID)
	if err != nil {
		return nil, fmt.Errorf("approval: %w", err)
	}

	approved, err := r.ApproveWith(*a, s.Policy)
	if err != nil {
		return nil, err
	}

	// Harness engineering gate: if a PreconditionRunner is wired
	// in, run the registered preconditions for the Capability
	// before promoting. A failing precondition blocks Activate and
	// surfaces a 409 with the per-hook output.
	if s.Harness != nil && s.HarnessDB != nil {
		precs, err := s.HarnessDB.ListPreconditionsForCapability(ctx, r.CapabilityID)
		if err != nil {
			return nil, fmt.Errorf("harness: list preconditions: %w", err)
		}
		if len(precs) > 0 {
			precsVal := make([]harness.Precondition, len(precs))
			for i, p := range precs {
				precsVal[i] = *p
			}
			if _, err := s.Harness.Run(ctx, precsVal); err != nil {
				return nil, err
			}
		}
	}

	now := s.Clock()
	activated, err := approved.Activate(now)
	if err != nil {
		return nil, err
	}

	// Supersede any prior Active Release in the same environment.
	prior, err := s.findPriorActive(ctx, r.CapabilityID, r.Environment)
	if err != nil {
		return nil, err
	}
	var superseded *Release
	if prior != nil {
		s, err := prior.Supersede(activated.ID, now)
		if err != nil {
			return nil, err
		}
		superseded = &s
		activated.ReplacesReleaseID = prior.ID
	}

	if err := s.DB.ActivateAtomic(ctx, superseded, &activated); err != nil {
		return nil, err
	}
	return &activated, nil
}

// Rollback moves an Active or Approved Release to RolledBack.
func (s *Service) Rollback(ctx context.Context, releaseID string) (*Release, error) {
	r, err := s.DB.GetRelease(ctx, releaseID)
	if err != nil {
		return nil, err
	}
	rolled, err := r.Rollback(s.Clock())
	if err != nil {
		return nil, err
	}
	if err := s.DB.UpdateRelease(ctx, &rolled); err != nil {
		return nil, err
	}
	return &rolled, nil
}

// Get returns a Release by ID.
func (s *Service) Get(ctx context.Context, releaseID string) (*Release, error) {
	return s.DB.GetRelease(ctx, releaseID)
}

// ListForCapability returns all Releases for a Capability.
func (s *Service) ListForCapability(ctx context.Context, capabilityID string) ([]*Release, error) {
	return s.DB.ListReleasesForCapability(ctx, capabilityID)
}

// ListActiveForEnvironment returns the active Releases in a given
// environment across all capabilities.
func (s *Service) ListActiveForEnvironment(ctx context.Context, env Environment) ([]*Release, error) {
	return s.DB.ListActiveReleasesForEnvironment(ctx, env)
}

// Approval returns the Approval trail for a Release.
func (s *Service) Approval(ctx context.Context, releaseID string) (*approval.Approval, error) {
	return s.App.GetApproval(ctx, releaseID)
}

func (s *Service) findPriorActive(ctx context.Context, capabilityID string, env Environment) (*Release, error) {
	releases, err := s.DB.ListReleasesForCapability(ctx, capabilityID)
	if err != nil {
		return nil, err
	}
	for _, r := range releases {
		if r.Environment == env && r.Status == StatusActive {
			return r, nil
		}
	}
	return nil, nil
}
