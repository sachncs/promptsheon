package release_test

import (
	"context"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/approval"
	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/release"
)

// memStore is a tiny in-memory Repository for service-level tests.
// SQLite-backed behaviour is covered in internal/store/sqlite_releases_test.go;
// here we exercise the Service's branching (Create, Vote, Activate,
// supersede, Rollback) against a fake.
type memStore struct {
	releases  map[string]*release.Release
	approvals map[string]*approval.Approval
}

func newMemStore() *memStore {
	return &memStore{
		releases:  make(map[string]*release.Release),
		approvals: make(map[string]*approval.Approval),
	}
}

func (m *memStore) CreateRelease(_ context.Context, r *release.Release) error {
	cp := *r
	m.releases[r.ID] = &cp
	return nil
}
func (m *memStore) GetRelease(_ context.Context, id string) (*release.Release, error) {
	r, ok := m.releases[id]
	if !ok {
		return nil, release.ErrNotFound
	}
	cp := *r
	return &cp, nil
}
func (m *memStore) ListReleasesForCapability(_ context.Context, capID string) ([]*release.Release, error) {
	var out []*release.Release
	for _, r := range m.releases {
		if r.CapabilityID == capID {
			cp := *r
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (m *memStore) ListActiveReleasesForEnvironment(_ context.Context, env release.Environment) ([]*release.Release, error) {
	var out []*release.Release
	for _, r := range m.releases {
		if r.Environment == env && r.Status == release.StatusActive {
			cp := *r
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (m *memStore) UpdateRelease(_ context.Context, r *release.Release) error {
	if _, ok := m.releases[r.ID]; !ok {
		return release.ErrNotFound
	}
	cp := *r
	m.releases[r.ID] = &cp
	return nil
}
func (m *memStore) DeleteRelease(_ context.Context, id string) error {
	delete(m.releases, id)
	return nil
}
func (m *memStore) CreateApproval(_ context.Context, a *approval.Approval) error {
	cp := *a
	m.approvals[a.ReleaseID] = &cp
	return nil
}
func (m *memStore) GetApproval(_ context.Context, releaseID string) (*approval.Approval, error) {
	a, ok := m.approvals[releaseID]
	if !ok {
		return nil, approval.ErrNotFound
	}
	cp := *a
	return &cp, nil
}
func (m *memStore) UpdateApproval(_ context.Context, a *approval.Approval) error {
	if _, ok := m.approvals[a.ReleaseID]; !ok {
		return approval.ErrNotFound
	}
	cp := *a
	m.approvals[a.ReleaseID] = &cp
	return nil
}
func (m *memStore) DeleteApproval(_ context.Context, releaseID string) error {
	delete(m.approvals, releaseID)
	return nil
}

func validManifest() capability.Manifest {
	h := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	return capability.Manifest{
		Prompt:        capability.ArtifactRef{Kind: capability.ArtifactPrompt, Hash: h},
		ModelPolicy:   capability.ArtifactRef{Kind: capability.ArtifactModelPolicy, Hash: h},
		RuntimePolicy: capability.ArtifactRef{Kind: capability.ArtifactRuntimePolicy, Hash: h},
		Context:       capability.ArtifactRef{Kind: capability.ArtifactContext, Hash: h},
		Memory:        capability.ArtifactRef{Kind: capability.ArtifactMemory, Hash: h},
	}
}

func newService(t *testing.T, kind release.PolicyKind, required int) (*release.Service, *memStore) {
	t.Helper()
	db := newMemStore()
	svc := release.NewServiceFromKind(db, db, kind, required)
	return svc, db
}

func TestServiceCreateVoteActivate(t *testing.T) {
	svc, _ := newService(t, release.PolicyMakerChecker, 1)
	ctx := context.Background()

	r, err := svc.Create(ctx, "c1", 1, validManifest(), release.EnvProd, "alice")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if r.Status != release.StatusPending {
		t.Fatalf("status = %q want pending", r.Status)
	}

	// bob approves (must be a non-creator identity)
	if _, err := svc.Vote(ctx, r.ID, approval.Vote{Identity: "bob", Decision: approval.Approve}); err != nil {
		t.Fatalf("vote: %v", err)
	}

	// activate
	activated, err := svc.Activate(ctx, r.ID)
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	if activated.Status != release.StatusActive {
		t.Fatalf("status = %q want active", activated.Status)
	}
	if activated.ActivatedAt == nil {
		t.Fatalf("activated_at should be set")
	}
}

func TestServiceActivateSupersedesPrior(t *testing.T) {
	svc, _ := newService(t, release.PolicyMakerChecker, 1)
	ctx := context.Background()

	r1, err := svc.Create(ctx, "c1", 1, validManifest(), release.EnvProd, "alice")
	if err != nil {
		t.Fatalf("create r1: %v", err)
	}
	if _, err := svc.Vote(ctx, r1.ID, approval.Vote{Identity: "bob", Decision: approval.Approve}); err != nil {
		t.Fatalf("vote r1: %v", err)
	}
	if _, err := svc.Activate(ctx, r1.ID); err != nil {
		t.Fatalf("activate r1: %v", err)
	}

	r2, err := svc.Create(ctx, "c1", 1, validManifest(), release.EnvProd, "alice")
	if err != nil {
		t.Fatalf("create r2: %v", err)
	}
	if _, err := svc.Vote(ctx, r2.ID, approval.Vote{Identity: "carol", Decision: approval.Approve}); err != nil {
		t.Fatalf("vote r2: %v", err)
	}
	if _, err := svc.Activate(ctx, r2.ID); err != nil {
		t.Fatalf("activate r2: %v", err)
	}

	got, err := svc.Get(ctx, r1.ID)
	if err != nil {
		t.Fatalf("get r1: %v", err)
	}
	if got.Status != release.StatusSuperseded || got.SupersededBy != r2.ID {
		t.Fatalf("r1 = %+v want Superseded/SupersededBy=%s", got.Status, r2.ID)
	}

	got2, err := svc.Get(ctx, r2.ID)
	if err != nil {
		t.Fatalf("get r2: %v", err)
	}
	if got2.ReplacesReleaseID != r1.ID {
		t.Fatalf("r2.replaces_release_id = %q want %q", got2.ReplacesReleaseID, r1.ID)
	}
}

func TestServiceActivateQuorumNotMet(t *testing.T) {
	svc, _ := newService(t, release.PolicyMakerChecker, 1)
	ctx := context.Background()

	r, err := svc.Create(ctx, "c1", 1, validManifest(), release.EnvProd, "alice")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// No votes at all — policy should reject.
	if _, err := svc.Activate(ctx, r.ID); err == nil {
		t.Fatalf("expected activate to fail without quorum")
	}
}

func TestServiceRollback(t *testing.T) {
	svc, _ := newService(t, release.PolicyMakerChecker, 1)
	ctx := context.Background()
	r, _ := svc.Create(ctx, "c1", 1, validManifest(), release.EnvProd, "alice")
	if _, err := svc.Vote(ctx, r.ID, approval.Vote{Identity: "bob", Decision: approval.Approve}); err != nil {
		t.Fatalf("vote: %v", err)
	}
	if _, err := svc.Activate(ctx, r.ID); err != nil {
		t.Fatalf("activate: %v", err)
	}
	rolled, err := svc.Rollback(ctx, r.ID)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if rolled.Status != release.StatusRolledBack {
		t.Fatalf("status = %q want rolled_back", rolled.Status)
	}
	if rolled.SupersededAt == nil {
		t.Fatalf("superseded_at should be set")
	}
}

func TestServiceClockSeam(t *testing.T) {
	svc, _ := newService(t, release.PolicyMakerChecker, 1)
	fixed := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	svc.Clock = func() time.Time { return fixed }
	ctx := context.Background()
	r, _ := svc.Create(ctx, "c1", 1, validManifest(), release.EnvProd, "alice")
	if _, err := svc.Vote(ctx, r.ID, approval.Vote{Identity: "bob", Decision: approval.Approve}); err != nil {
		t.Fatalf("vote: %v", err)
	}
	activated, err := svc.Activate(ctx, r.ID)
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	if !activated.ActivatedAt.Equal(fixed) {
		t.Fatalf("activated_at = %v want %v", activated.ActivatedAt, fixed)
	}
}
