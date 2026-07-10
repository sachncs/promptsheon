package approval

import (
	"errors"
	"testing"
	"time"
)

func vote(identity string, d Decision, reason string) Vote {
	return Vote{Identity: identity, Decision: d, Reason: reason, Timestamp: time.Now()}
}

func TestRecordRejectsEmptyIdentity(t *testing.T) {
	t.Parallel()
	a := Approval{}
	if _, err := a.Record(vote("", Approve, "")); err == nil {
		t.Fatalf("expected error for empty identity")
	}
}

func TestRecordRejectsUnknownDecision(t *testing.T) {
	t.Parallel()
	a := Approval{}
	if _, err := a.Record(vote("alice", Decision("maybe"), "")); !errors.Is(err, ErrUnknownDecision) {
		t.Fatalf("expected ErrUnknownDecision, got %v", err)
	}
}

func TestRecordRejectsDuplicateIdentity(t *testing.T) {
	t.Parallel()
	a := Approval{}
	a, _ = a.Record(vote("alice", Approve, ""))
	if _, err := a.Record(vote("alice", Approve, "oops")); !errors.Is(err, ErrDuplicateIdentity) {
		t.Fatalf("expected ErrDuplicateIdentity, got %v", err)
	}
}

func TestRecordAppends(t *testing.T) {
	t.Parallel()
	a := Approval{ReleaseID: "rel-1"}
	a, err := a.Record(vote("alice", Approve, ""))
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	a, err = a.Record(vote("bob", Approve, ""))
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if len(a.Votes) != 2 {
		t.Fatalf("expected 2 votes, got %d", len(a.Votes))
	}
}

func TestMajorityPolicy(t *testing.T) {
	t.Parallel()
	pol := MajorityPolicy{Required: 2}

	got, ok, err := pol.Evaluate([]Vote{vote("alice", Approve, "")})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if ok || got != StatePending {
		t.Fatalf("expected Pending with 1/2, got %s ok=%v", got, ok)
	}

	got, ok, err = pol.Evaluate([]Vote{vote("alice", Approve, ""), vote("bob", Approve, "")})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !ok || got != StateApproved {
		t.Fatalf("expected Approved at 2/2, got %s ok=%v", got, ok)
	}
}

func TestMajorityPolicyRejectHalts(t *testing.T) {
	t.Parallel()
	pol := MajorityPolicy{Required: 2}
	got, ok, err := pol.Evaluate([]Vote{
		vote("alice", Approve, ""),
		vote("bob", Reject, "no good"),
		vote("carol", Approve, ""),
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if ok || got != StateRejected {
		t.Fatalf("expected Rejected, got %s ok=%v", got, ok)
	}
}

func TestMajorityPolicyRejectsNonPositiveRequired(t *testing.T) {
	t.Parallel()
	if _, _, err := (MajorityPolicy{}).Evaluate(nil); err == nil {
		t.Fatalf("expected error for Required=0")
	}
}

func TestMakerCheckerRequiresSeparation(t *testing.T) {
	t.Parallel()
	a := Approval{ReleaseID: "rel-1"}
	a, _ = a.Record(vote("alice", Approve, ""))
	if VerifySeparationOfDuties(a, "alice") {
		t.Fatalf("expected maker to fail separation check")
	}
	if !VerifySeparationOfDuties(a, "bob") {
		t.Fatalf("expected non-maker to pass separation check")
	}
	if VerifySeparationOfDuties(a, "") {
		t.Fatalf("expected empty creator to fail separation check")
	}
}
