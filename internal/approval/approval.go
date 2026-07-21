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

// ErrDuplicateIdentity is returned when the same identity tries to
// vote twice on the same Release.
var ErrDuplicateIdentity = errors.New("approval: duplicate voter")

// ErrCreatorVoted is returned by VerifySeparationOfDuties when the
// Release creator is found in the votes list.
var ErrCreatorVoted = errors.New("approval: creator voted on own release (separation of duties)")

// ErrQuorumNotSatisfied is returned when the Policy reports the
// supplied votes do not yet constitute an Approved state.
var ErrQuorumNotSatisfied = errors.New("approval: quorum not yet satisfied")

// ErrUnknownDecision is returned when a decision value is not one of
// the supported closed-set values.
var ErrUnknownDecision = errors.New("approval: unknown decision")

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
		return a, fmt.Errorf("%w: %q", ErrUnknownDecision, v.Decision)
	}
	for _, existing := range a.Votes {
		if existing.Identity == v.Identity {
			return a, ErrDuplicateIdentity
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
// enforced inside Evaluate itself: any vote whose identity matches
// Creator is rejected with ErrCreatorVoted. Callers no longer need
// to invoke a side-check helper.
//
// Empty Creator is treated conservatively (the policy refuses to
// evaluate) so a misconfigured caller cannot accidentally bypass the
// rule. This is the SEC-1 fix.
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
			return "", false, ErrCreatorVoted
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

// VerifySeparationOfDuties is retained for backward compatibility
// with callers that already check before Evaluate. New code should
// pass Creator into MakerCheckerPolicy and let Evaluate decide.
func VerifySeparationOfDuties(a Approval, creator string) bool {
	if creator == "" {
		return false
	}
	for _, v := range a.Votes {
		if v.Identity == creator {
			return false
		}
	}
	return true
}
