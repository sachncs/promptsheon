// Package approval defines the Approval aggregate and the quorum
// policy that decides when a Release is ready to be activated.
//
// Approval is intentionally separate from Release. A Release holds the
// pointer to a Version; an Approval holds the decision-making trail.
// Splitting them lets us record multiple approvals from independent
// identities, change quorum rules without rewriting Release history,
// and audit approvers across Releases without paging through Release
// rows.
package approval

import (
	"github.com/sachncs/promptsheon/backend/errs"
	"errors"
	"fmt"
	"time"

)

// Decision is the per-identity vote on a Release.
type Decision string

const (
	// Approve is a vote to advance the Release.
	Approve Decision = "approve"
	// Reject is a veto. Any Reject before the quorum is met holds
	// the Release in Pending state; once recorded, the same Release
	// can never be approved without a new Release being created.
	Reject Decision = "reject"
	// Abstain records participation without advancing or vetoing.
	Abstain Decision = "abstain"
)

// Vote is one identity's recorded position on a Release.
//
// Approval is value-immutable: recording a vote returns a new Approval
// with the vote appended.
type Vote struct {
	Identity  string    `json:"identity"`
	Decision  Decision  `json:"decision"`
	Reason    string    `json:"reason,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Policy decides whether a set of Votes satisfies the quorum
// requirement for a Release.
//
// Policy is intentionally not part of the Release; quorum rules evolve
// faster than Releases do.
type Policy interface {
	// Evaluate returns the resulting state of the Approval. The
	// boolean is true if and only if the quorum is satisfied and
	// the Release should advance.
	Evaluate(votes []Vote) (state State, satisfied bool, err error)
}

// State is the aggregate outcome of the votes so far.
type State string

const (
	StatePending  State = "pending"
	StateApproved State = "approved"
	StateRejected State = "rejected"
)

// Approval is the trail of votes against a Release.
type Approval struct {
	ReleaseID string    `json:"release_id"`
	Votes     []Vote    `json:"votes"`
	UpdatedAt time.Time `json:"updated_at"`
}

// errs.ErrorApprovalDuplicateIdentity is returned when the same identity tries to
// vote twice on the same Release.

// errs.ErrorApprovalCreatorVoted is returned by MakerCheckerPolicy.Evaluate when
// the Release creator is found in the votes list. The policy
// owns the separation-of-duties check; no separate helper is
// required.

// errs.ErrorApprovalQuorumNotMet is returned when the Policy reports the
// supplied votes do not yet constitute an Approved state.

// errs.ErrorApprovalUnknownDecision is returned when a decision value is not one of
// the supported closed-set values.

// Record adds a vote to the Approval and returns a new value. The
// caller is expected to re-evaluate the Policy after each Record.
//
// Record rejects duplicates by identity — one person, one vote per
// Release — which is the smallest defense against an approver voting
// twice on the same Release.
func (a Approval) Record(v Vote) (Approval, error) {
	if v.Identity == "" {
		return a, errors.New("approval: voter identity is required")
	}
	switch v.Decision {
	case Approve, Reject, Abstain:
	default:
		return a, fmt.Errorf("%w: %q", errs.ErrorApprovalUnknownDecision, v.Decision)
	}
	for _, existing := range a.Votes {
		if existing.Identity == v.Identity {
			return a, errs.ErrorApprovalDuplicateIdentity
		}
	}
	if v.Timestamp.IsZero() {
		v.Timestamp = time.Now().UTC()
	}
	a.Votes = append(append([]Vote{}, a.Votes...), v)
	a.UpdatedAt = v.Timestamp
	return a, nil
}

// MajorityPolicy is a Policy that requires an absolute number of
// Approve votes, with any Reject holding the Release in Rejected.
type MajorityPolicy struct {
	Required int
}

// Evaluate implements Policy.
//
// A Reject at any time sends the state to Rejected. Once the
// Required number of Approve votes is recorded, the state advances to
// Approved. Abstain votes count as participation only.
func (p MajorityPolicy) Evaluate(votes []Vote) (State, bool, error) {
	if p.Required <= 0 {
		return "", false, errors.New("approval: MajorityPolicy.Required must be positive")
	}
	for _, v := range votes {
		if v.Decision == Reject {
			return StateRejected, false, nil
		}
	}
	approves := 0
	for _, v := range votes {
		if v.Decision == Approve {
			approves++
		}
	}
	if approves >= p.Required {
		return StateApproved, true, nil
	}
	return StatePending, false, nil
}

// MakerCheckerPolicy requires the creator to abstain and at least one
// other identity to Approve. The separation-of-duties rule is
// enforced inside Evaluate: any vote whose Identity matches Creator
// fails the entire evaluation with errs.ErrorApprovalCreatorVoted (the rejected
// state is not advanced). Callers no longer need to invoke a
// side-check helper.
//
// DOC-1: the failure modes are fail-closed. Evaluate returns
// (State, false, err) on every misconfiguration:
//
//   - RequiredApprovers <= 0  →  error "RequiredApprovers must be positive"
//   - Creator == ""           →  error "Creator is required for maker-checker enforcement"
//   - any vote by Creator     →  error errs.ErrorApprovalCreatorVoted
//
// The policy will NEVER report StateApproved without a
// non-creator Approve vote. The previous version silently
// approved when Creator was empty; the SEC-1 fix closes that
// gap by refusing to evaluate.
type MakerCheckerPolicy struct {
	RequiredApprovers int
	Creator           string
}

// Evaluate implements Policy.
func (p MakerCheckerPolicy) Evaluate(votes []Vote) (State, bool, error) {
	if p.RequiredApprovers <= 0 {
		return "", false, errors.New("approval: MakerCheckerPolicy.RequiredApprovers must be positive")
	}
	if p.Creator == "" {
		return "", false, errors.New("approval: MakerCheckerPolicy.Creator is required for maker-checker enforcement")
	}
	for _, v := range votes {
		if v.Identity == p.Creator {
			return "", false, errs.ErrorApprovalCreatorVoted
		}
		if v.Decision == Reject {
			return StateRejected, false, nil
		}
	}
	approves := 0
	for _, v := range votes {
		if v.Decision == Approve {
			approves++
		}
	}
	if approves >= p.RequiredApprovers {
		return StateApproved, true, nil
	}
	return StatePending, false, nil
}

// SelfApprovePolicy auto-approves a Release when the supplied
// votes contain at least one Approve from the configured
// SelfApprover identity. The policy intentionally bypasses the
// maker-checker separation-of-duties check: the evolver is the
// only caller and it has already validated the revision before
// asking for activation. The self-approver identity is recorded
// in the audit chain so operators can trace every auto-promote.
//
// RequiredApprovers defaults to 1. SelfApprover is the identity
// the policy trusts; an empty SelfApprover makes the policy
// always report pending, which is the safe default if the
// evolver is misconfigured.
type SelfApprovePolicy struct {
	RequiredApprovers int
	SelfApprover      string
}

// Evaluate implements Policy. Returns StateApproved once one or
// more Approve votes from SelfApprover accumulate.
func (p SelfApprovePolicy) Evaluate(votes []Vote) (State, bool, error) {
	required := p.RequiredApprovers
	if required <= 0 {
		required = 1
	}
	if p.SelfApprover == "" {
		return StatePending, false, errors.New("approval: SelfApprovePolicy.SelfApprover is required")
	}
	for _, v := range votes {
		if v.Decision == Reject {
			return StateRejected, false, nil
		}
	}
	approves := 0
	for _, v := range votes {
		if v.Decision == Approve && v.Identity == p.SelfApprover {
			approves++
		}
	}
	if approves >= required {
		return StateApproved, true, nil
	}
	return StatePending, false, nil
}

// VerifySeparationOfDuties was removed in SEC-1b. MakerCheckerPolicy
// self-enforces against the Creator field at Evaluate time;
// callers should populate Creator on the policy before passing
// it to Evaluate.
