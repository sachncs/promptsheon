// Package release defines the Release aggregate.
//
// A Release is the approved pointer from a Capability Version to a
// target Environment. It exists separately from CapabilityVersion
// (which is purely the implementation) and from Deployment (which is
// the operational act of placing that Version into production traffic
// in one Environment). The lifecycle is:
//
//	Version (immutable) -> Approval -> Release -> Deployment
//
// Release is immutable once its `Status` moves to `Active`. Rollback
// does not modify a Release; it creates a successor Release that
// re-points the Environment to a prior Version.
package release

import (
	"errors"
	"fmt"
	"time"

	"github.com/sachncs/promptsheon/internal/approval"
	"github.com/sachncs/promptsheon/internal/capability"
)

// Status is the lifecycle state of a Release.
type Status string

const (
	// StatusPending is awaiting Approval before it can be deployed.
	StatusPending Status = "pending"
	// StatusApproved has been approved but not yet deployed.
	StatusApproved Status = "approved"
	// StatusActive is the Release currently serving traffic in its
	// Environment.
	StatusActive Status = "active"
	// StatusSuperseded is a Release that was once active but has
	// been replaced by a later Release in the same Environment.
	// Superseded Releases are preserved for audit and rollback.
	StatusSuperseded Status = "superseded"
	// StatusRolledBack was active and then explicitly rolled back.
	StatusRolledBack Status = "rolled_back"
)

// Environment is the target environment for a Release.
//
// Environments are deliberately a small closed set: there is exactly
// one "active" Release per Environment per Capability at a time. New
// Environments are added by code change, not by data, so the set is
// auditable.
type Environment string

const (
	EnvDev     Environment = "dev"
	EnvStaging Environment = "staging"
	EnvProd    Environment = "prod"
)

// Valid reports whether the environment is one of the supported
// closed-set values.
func (e Environment) Valid() bool {
	switch e {
	case EnvDev, EnvStaging, EnvProd:
		return true
	default:
		return false
	}
}

// ErrNotPending is returned when a transition is attempted on a
// Release that is not in the Pending state.
var ErrNotPending = errors.New("release: transition requires Pending status")

// ErrUnknownEnvironment is returned when an Environment fails the
// closed-set check.
var ErrUnknownEnvironment = errors.New("release: unknown environment")

// Release is the approved pointer from a Version to an Environment.
//
// A Release is constructed in Pending state, moves to Approved when
// the approval quorum is satisfied, and becomes Active when its
// Deployment succeeds. Rollback produces a successor Release; this one
// transitions to Superseded or RolledBack depending on the cause.
//
// Release is a value type. Methods that "transition" a Release return
// a new value rather than mutating in place, in keeping with the
// immutability principle for domain objects.
type Release struct {
	ID                string              `json:"id"`
	CapabilityID      string              `json:"capability_id"`
	CapabilityVersion int                 `json:"capability_version"`
	Manifest          capability.Manifest `json:"manifest"`
	Environment       Environment         `json:"environment"`
	Status            Status              `json:"status"`
	ApprovedBy        []string            `json:"approved_by,omitempty"`
	SupersededBy      string              `json:"superseded_by,omitempty"`
	ReplacesReleaseID string              `json:"replaces_release_id,omitempty"`
	CreatedAt         time.Time           `json:"created_at"`
	CreatedBy         string              `json:"created_by"`
	ActivatedAt       *time.Time          `json:"activated_at,omitempty"`
	SupersededAt      *time.Time          `json:"superseded_at,omitempty"`
}

// New constructs a Pending Release. Callers are responsible for
// generating IDs; the function does not allocate one.
//
// Manifest.Validate must succeed before a Release is constructed. We
// re-validate here as a defense-in-depth check rather than rely on the
// caller.
func New(capabilityID string, capabilityVersion int, manifest capability.Manifest, environment Environment, createdBy string) (Release, error) {
	if capabilityID == "" {
		return Release{}, errors.New("release: capability_id is required")
	}
	if createdBy == "" {
		return Release{}, errors.New("release: created_by is required")
	}
	if !environment.Valid() {
		return Release{}, fmt.Errorf("%w: %q", ErrUnknownEnvironment, environment)
	}
	if err := manifest.Validate(); err != nil {
		return Release{}, fmt.Errorf("release: manifest: %w", err)
	}
	return Release{
		CapabilityID:      capabilityID,
		CapabilityVersion: capabilityVersion,
		Manifest:          manifest,
		Environment:       environment,
		Status:            StatusPending,
		CreatedBy:         createdBy,
		CreatedAt:         time.Now().UTC(),
	}, nil
}

// ApproveWith transitions a Pending Release to Approved by reading
// the supplied *approval.Approval and consulting its Policy. When
// the Policy reports the quorum satisfied AND no pending Reject, the
// Release moves to Approved and the approvers list is recorded.
//
// MakerCheckerPolicy enforces separation-of-duties internally by
// comparing each vote's identity against the Release's CreatedBy.
// The caller no longer needs to invoke VerifySeparationOfDuties
// as a separate step (SEC-1): the policy is self-checking.
func (r Release) ApproveWith(a approval.Approval, pol approval.Policy) (Release, error) {
	if r.Status != StatusPending {
		return r, ErrNotPending
	}
	if r.CreatedBy == "" {
		return r, errors.New("release: cannot approve; created_by is empty (cannot run separation-of-duties check)")
	}
	// For MakerCheckerPolicy, populate the policy's Creator field
	// from the Release so the policy can self-enforce. For other
	// policies, the wrapped Evaluate retains the legacy behaviour
	// and the side-check below still runs.
	if mkp, ok := pol.(approval.MakerCheckerPolicy); ok {
		mkp.Creator = r.CreatedBy
		pol = mkp
	}
	if !approval.VerifySeparationOfDuties(a, r.CreatedBy) {
		return r, approval.ErrCreatorVoted
	}
	state, satisfied, err := pol.Evaluate(a.Votes)
	if err != nil {
		return r, fmt.Errorf("release: policy: %w", err)
	}
	if !satisfied {
		return r, fmt.Errorf("%w: state=%s, votes=%d", approval.ErrQuorumNotSatisfied, state, len(a.Votes))
	}
	for _, v := range a.Votes {
		if v.Decision == approval.Approve {
			r.ApprovedBy = append(r.ApprovedBy, v.Identity)
		}
	}
	r.Status = StatusApproved
	return r, nil
}

// ApproveWithApprovalList is a convenience for callers that have not
// (yet) wired up the full Approval package. It accepts a flat list of
// approver identities and uses the built-in MajorityPolicy with
// Required=len(approvers). Empty identities are rejected before the
// Approval is constructed so a release cannot be Approved with a
// blank voter.
//
// Approve returns ErrNotPending if the Release is already past Pending.
func (r Release) ApproveWithApprovalList(approvers []string) (Release, error) {
	for _, a := range approvers {
		if a == "" {
			return r, errors.New("release: approver identity must not be empty")
		}
	}
	return r.ApproveWith(
		approval.Approval{ReleaseID: r.ID, Votes: castApprovesToVotes(approvers)},
		approval.MajorityPolicy{Required: len(approvers)},
	)
}

// Deprecated: removed in the DEAD-3 sweep. Use
// ApproveWith(approval.Approval, approval.Policy) instead.

func castApprovesToVotes(identities []string) []approval.Vote {
	out := make([]approval.Vote, 0, len(identities))
	now := time.Now().UTC()
	for _, id := range identities {
		out = append(out, approval.Vote{
			Identity:  id,
			Decision:  approval.Approve,
			Timestamp: now,
		})
	}
	return out
}

// Activate transitions an Approved Release to Active.
// The caller passes the activation time so clocks are explicit at
// boundaries (test seams, replay buffers).
func (r Release) Activate(at time.Time) (Release, error) {
	if r.Status != StatusApproved {
		return r, fmt.Errorf("release: activate requires Approved status, got %s", r.Status)
	}
	r.Status = StatusActive
	r.ActivatedAt = &at
	return r, nil
}

// Supersede records that this Release has been replaced by another
// Release in the same Environment. The replacement is identified so
// the lineage of Releases can be reconstructed.
//
// SupersededAt is set to the supplied time; again clocks are explicit.
func (r Release) Supersede(byReleaseID string, at time.Time) (Release, error) {
	if r.Status != StatusActive {
		return r, fmt.Errorf("release: supersede requires Active status, got %s", r.Status)
	}
	if byReleaseID == "" {
		return r, errors.New("release: supersede requires replacement release id")
	}
	r.Status = StatusSuperseded
	r.SupersededBy = byReleaseID
	r.SupersededAt = &at
	return r, nil
}

// Rollback records that this Release was rolled back. The cause is
// preserved as a free-text reason for audit; the actual cause code is
// the domain of an incident, not this aggregate.
func (r Release) Rollback(at time.Time) (Release, error) {
	if r.Status != StatusActive && r.Status != StatusApproved {
		return r, fmt.Errorf("release: rollback requires Active or Approved status, got %s", r.Status)
	}
	r.Status = StatusRolledBack
	t := at
	r.SupersededAt = &t
	return r, nil
}
