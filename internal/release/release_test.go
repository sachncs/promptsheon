package release

import (
	"errors"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
)

func goodManifest() capability.Manifest {
	h := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	return capability.Manifest{
		Prompt:        capability.ArtifactRef{Kind: capability.ArtifactPrompt, Hash: h},
		ModelPolicy:   capability.ArtifactRef{Kind: capability.ArtifactModelPolicy, Hash: h},
		RuntimePolicy: capability.ArtifactRef{Kind: capability.ArtifactRuntimePolicy, Hash: h},
		Context:       capability.ArtifactRef{Kind: capability.ArtifactContext, Hash: h},
		Memory:        capability.ArtifactRef{Kind: capability.ArtifactMemory, Hash: h},
	}
}

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
	got, err := r.Approve([]string{"bob", "carol"})
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
	r, _ = r.Approve([]string{"bob"})
	if _, err := r.Approve([]string{"carol"}); !errors.Is(err, ErrNotPending) {
		t.Fatalf("expected ErrNotPending, got %v", err)
	}
}

func TestApproveRejectsEmptyApprovers(t *testing.T) {
	t.Parallel()
	r, _ := New("cap", 1, goodManifest(), EnvProd, "alice")
	if _, err := r.Approve(nil); err == nil {
		t.Fatalf("expected error for empty approvers")
	}
	if _, err := r.Approve([]string{""}); err == nil {
		t.Fatalf("expected error for empty approver identity")
	}
}

func TestActivateRequiresApproved(t *testing.T) {
	t.Parallel()
	r, _ := New("cap", 1, goodManifest(), EnvProd, "alice")
	if _, err := r.Activate(time.Now()); err == nil {
		t.Fatalf("expected error activating Pending release")
	}
	r, _ = r.Approve([]string{"bob"})
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
	r, _ = r.Approve([]string{"bob"})
	got, err := r.Rollback(time.Now())
	if err != nil {
		t.Fatalf("rollback from Approved: %v", err)
	}
	if got.Status != StatusRolledBack {
		t.Fatalf("expected RolledBack, got %s", got.Status)
	}
}
