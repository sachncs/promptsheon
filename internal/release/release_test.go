package release

import (
	"errors"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/approval"
	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/testdata"
)

func goodManifest() capability.Manifest { return testdata.NewManifest() }

func TestNew(t *testing.T) {
	t.Parallel()
	r, err := New("cap-1", 7, goodManifest(), EnvProd, "alice")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if r.Status != StatusPending {
		t.Fatalf("expected Pending, got %s", r.Status)
	}
	if r.Environment != EnvProd {
		t.Fatalf("expected prod, got %s", r.Environment)
	}
}

func TestNewRejectsBadInput(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		fn   func() (Release, error)
	}{
		{"empty capability_id", func() (Release, error) { return New("", 1, goodManifest(), EnvProd, "alice") }},
		{"empty created_by", func() (Release, error) { return New("cap", 1, goodManifest(), EnvProd, "") }},
		{"unknown env", func() (Release, error) { return New("cap", 1, goodManifest(), Environment("preprod"), "alice") }},
		{"invalid manifest", func() (Release, error) { return New("cap", 1, capability.Manifest{}, EnvProd, "alice") }},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := tc.fn(); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

func TestApproveAdvancesPending(t *testing.T) {
	t.Parallel()
	r, _ := New("cap", 1, goodManifest(), EnvProd, "alice")
	a, err := approval.Approval{ReleaseID: r.ID}.Record(
		approval.Vote{Identity: "bob", Decision: approval.Approve, Timestamp: time.Now()},
	)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	a, _ = a.Record(approval.Vote{Identity: "carol", Decision: approval.Approve, Timestamp: time.Now()})
	got, err := r.ApproveWith(a, approval.MajorityPolicy{Required: 2})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if got.Status != StatusApproved {
		t.Fatalf("expected Approved, got %s", got.Status)
	}
	if len(got.ApprovedBy) != 2 {
		t.Fatalf("expected 2 approvers, got %d", len(got.ApprovedBy))
	}
}

func TestApproveRejectsWrongState(t *testing.T) {
	t.Parallel()
	r, _ := New("cap", 1, goodManifest(), EnvProd, "alice")
	a, _ := approval.Approval{ReleaseID: r.ID}.Record(
		approval.Vote{Identity: "bob", Decision: approval.Approve, Timestamp: time.Now()},
	)
	r, _ = r.ApproveWith(a, approval.MajorityPolicy{Required: 1})
	if _, err := r.ApproveWith(a, approval.MajorityPolicy{Required: 1}); !errors.Is(err, ErrNotPending) {
		t.Fatalf("expected ErrNotPending, got %v", err)
	}
}

func TestApproveRejectsEmptyApprovers(t *testing.T) {
	t.Parallel()
	r, _ := New("cap", 1, goodManifest(), EnvProd, "alice")
	empty, _ := approval.Approval{ReleaseID: r.ID}.Record(
		approval.Vote{Identity: "", Decision: approval.Approve, Timestamp: time.Now()},
	)
	if _, err := r.ApproveWith(empty, approval.MajorityPolicy{Required: 1}); err == nil {
		t.Fatalf("expected error for empty approver identity")
	}
}

func TestActivateRequiresApproved(t *testing.T) {
	t.Parallel()
	r, _ := New("cap", 1, goodManifest(), EnvProd, "alice")
	if _, err := r.Activate(time.Now()); err == nil {
		t.Fatalf("expected error activating Pending release")
	}
	a, _ := approval.Approval{ReleaseID: r.ID}.Record(
		approval.Vote{Identity: "bob", Decision: approval.Approve, Timestamp: time.Now()},
	)
	r, _ = r.ApproveWith(a, approval.MajorityPolicy{Required: 1})
	got, err := r.Activate(time.Now())
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	if got.Status != StatusActive {
		t.Fatalf("expected Active, got %s", got.Status)
	}
}

func TestSupersedeRequiresActive(t *testing.T) {
	t.Parallel()
	r, _ := New("cap", 1, goodManifest(), EnvProd, "alice")
	if _, err := r.Supersede("rel-2", time.Now()); err == nil {
		t.Fatalf("expected error superseding non-Active")
	}
}

func TestRollbackFromActiveOrApproved(t *testing.T) {
	t.Parallel()
	r, _ := New("cap", 1, goodManifest(), EnvProd, "alice")
	a, _ := approval.Approval{ReleaseID: r.ID}.Record(
		approval.Vote{Identity: "bob", Decision: approval.Approve, Timestamp: time.Now()},
	)
	r, _ = r.ApproveWith(a, approval.MajorityPolicy{Required: 1})
	got, err := r.Rollback(time.Now())
	if err != nil {
		t.Fatalf("rollback from Approved: %v", err)
	}
	if got.Status != StatusRolledBack {
		t.Fatalf("expected RolledBack, got %s", got.Status)
	}
}

func TestApproveWithEnforcesQuorum(t *testing.T) {
	t.Parallel()
	r, _ := New("cap", 1, goodManifest(), EnvProd, "alice")
	a, err := approval.Approval{ReleaseID: r.ID}.Record(
		approval.Vote{Identity: "bob", Decision: approval.Approve, Timestamp: time.Now()},
	)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	// Required=2, only one Approve vote: quorum not yet satisfied.
	_, err = r.ApproveWith(a, approval.MajorityPolicy{Required: 2})
	if err == nil {
		t.Fatalf("expected ErrQuorumNotSatisfied, got nil")
	}
	// Second Approve reaches quorum.
	a, _ = a.Record(approval.Vote{Identity: "carol", Decision: approval.Approve, Timestamp: time.Now()})
	out, err := r.ApproveWith(a, approval.MajorityPolicy{Required: 2})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if out.Status != StatusApproved {
		t.Fatalf("expected Approved, got %s", out.Status)
	}
	if len(out.ApprovedBy) != 2 {
		t.Fatalf("expected 2 approvers recorded, got %d", len(out.ApprovedBy))
	}
}

func TestApproveWithRejectsCreatorVote(t *testing.T) {
	t.Parallel()
	// SEC-1b: separation-of-duties lives on MakerCheckerPolicy, not
	// on the release layer. MajorityPolicy intentionally allows the
	// creator's own vote.
	r, _ := New("cap", 1, goodManifest(), EnvProd, "alice")
	a, _ := approval.Approval{ReleaseID: r.ID}.Record(
		approval.Vote{Identity: "alice", Decision: approval.Approve, Timestamp: time.Now()},
	)
	_, err := r.ApproveWith(a, approval.MakerCheckerPolicy{RequiredApprovers: 1})
	if err == nil {
		t.Fatalf("expected maker-checker to reject creator's own vote")
	}
}

func TestApproveWithStopsOnReject(t *testing.T) {
	t.Parallel()
	r, _ := New("cap", 1, goodManifest(), EnvProd, "alice")
	a, _ := approval.Approval{ReleaseID: r.ID}.Record(
		approval.Vote{Identity: "bob", Decision: approval.Reject, Timestamp: time.Now()},
	)
	_, err := r.ApproveWith(a, approval.MajorityPolicy{Required: 1})
	if err == nil {
		t.Fatalf("expected rejection error")
	}
}

func TestApproveWithMakerCheckerPolicy(t *testing.T) {
	t.Parallel()
	r, _ := New("cap", 1, goodManifest(), EnvProd, "alice")
	a, _ := approval.Approval{ReleaseID: r.ID}.Record(
		approval.Vote{Identity: "bob", Decision: approval.Approve, Timestamp: time.Now()},
	)
	out, err := r.ApproveWith(a, approval.MakerCheckerPolicy{RequiredApprovers: 1})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if out.Status != StatusApproved {
		t.Fatalf("expected Approved, got %s", out.Status)
	}
}

func TestMakerCheckerPolicySelfEnforcesCreator(t *testing.T) {
	t.Parallel()
	r, _ := New("cap", 1, goodManifest(), EnvProd, "alice")
	// alice (the creator) votes Approve. The policy must reject
	// even when the side-check has not been called.
	a, _ := approval.Approval{ReleaseID: r.ID}.Record(
		approval.Vote{Identity: "alice", Decision: approval.Approve, Timestamp: time.Now()},
	)
	_, err := r.ApproveWith(a, approval.MakerCheckerPolicy{RequiredApprovers: 1, Creator: "alice"})
	if !errors.Is(err, approval.ErrCreatorVoted) {
		t.Fatalf("expected ErrCreatorVoted, got %v", err)
	}
}

func TestMakerCheckerPolicyEmptyCreatorRejected(t *testing.T) {
	// Direct policy test: the side-check is not on the call path,
	// so the policy itself must refuse to evaluate without a creator.
	_, _, err := approval.MakerCheckerPolicy{RequiredApprovers: 1}.Evaluate([]approval.Vote{
		{Identity: "bob", Decision: approval.Approve, Timestamp: time.Now()},
	})
	if err == nil {
		t.Fatal("expected error when Creator is empty")
	}
}
