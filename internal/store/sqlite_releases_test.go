package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/approval"
	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/release"
	"github.com/sachncs/promptsheon/internal/store"
)

// fixture provides a populated workspace/project/capability/version so
// the release tests have something to point at.
type releaseFixture struct {
	db            *store.SQLite
	workspaceID   string
	projectID     string
	capabilityID  string
	versionID     string
	manifestHash  string
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

func newReleaseFixture(t *testing.T) *releaseFixture {
	t.Helper()
	db, err := store.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	now := time.Now().UTC()

	w := &capability.Workspace{ID: "w1", Name: "test", CreatedAt: now, UpdatedAt: now}
	if err := db.CreateWorkspace(ctx, w); err != nil {
		t.Fatalf("workspace: %v", err)
	}
	p := &capability.Project{ID: "p1", WorkspaceID: "w1", Name: "test", CreatedAt: now, UpdatedAt: now}
	if err := db.CreateProject(ctx, p); err != nil {
		t.Fatalf("project: %v", err)
	}
	c := &capability.Capability{ID: "c1", ProjectID: "p1", Name: "greeting", CreatedAt: now, UpdatedAt: now}
	if err := db.CreateCapability(ctx, c); err != nil {
		t.Fatalf("capability: %v", err)
	}
	manifest := capability.Manifest{}
	v := &capability.Version{
		ID: "v1", CapabilityID: "c1", Version: 1, Manifest: manifest,
		ManifestHash: "h1", CreatedAt: now, CreatedBy: "alice",
	}
	if err := db.CreateVersion(ctx, v); err != nil {
		t.Fatalf("version: %v", err)
	}
	return &releaseFixture{
		db: db, workspaceID: "w1", projectID: "p1",
		capabilityID: "c1", versionID: "v1", manifestHash: "h1",
	}
}

func TestReleaseCreateGetRoundTrip(t *testing.T) {
	fx := newReleaseFixture(t)
	before := time.Now().UTC()
	rel, err := release.New(fx.capabilityID, 1, validManifest(), release.EnvProd, "alice")
	if err != nil {
		t.Fatalf("new release: %v", err)
	}
	rel.ID = "r1"

	if err := fx.db.CreateRelease(context.Background(), &rel); err != nil {
		t.Fatalf("create release: %v", err)
	}

	got, err := fx.db.GetRelease(context.Background(), "r1")
	if err != nil {
		t.Fatalf("get release: %v", err)
	}
	if got.CapabilityID != fx.capabilityID {
		t.Fatalf("capability_id = %q want %q", got.CapabilityID, fx.capabilityID)
	}
	if got.Environment != release.EnvProd {
		t.Fatalf("env = %q want prod", got.Environment)
	}
	if got.Status != release.StatusPending {
		t.Fatalf("status = %q want pending", got.Status)
	}
	// SQLite stores times at second precision; allow a 2s window.
	if got.CreatedAt.Before(before.Add(-2 * time.Second)) || got.CreatedAt.After(time.Now().UTC().Add(2*time.Second)) {
		t.Fatalf("created_at = %v out of expected window", got.CreatedAt)
	}
}

func TestReleaseActivateSupersedes(t *testing.T) {
	fx := newReleaseFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()

	r1, _ := release.New(fx.capabilityID, 1, validManifest(), release.EnvProd, "alice")
	r1.ID = "r1"
	r2, _ := release.New(fx.capabilityID, 1, validManifest(), release.EnvProd, "alice")
	r2.ID = "r2"

	if err := fx.db.CreateRelease(ctx, &r1); err != nil {
		t.Fatalf("create r1: %v", err)
	}
	if err := fx.db.CreateRelease(ctx, &r2); err != nil {
		t.Fatalf("create r2: %v", err)
	}

	// Approve+activate r1
	a := &approval.Approval{ReleaseID: "r1", Votes: []approval.Vote{
		{Identity: "bob", Decision: approval.Approve, Timestamp: now},
	}, UpdatedAt: now}
	if err := fx.db.CreateApproval(ctx, a); err != nil {
		t.Fatalf("create approval: %v", err)
	}
	r1Approved, err := r1.ApproveWith(*a, approval.MakerCheckerPolicy{RequiredApprovers: 1})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if err := fx.db.UpdateApproval(ctx, a); err != nil {
		t.Fatalf("update approval: %v", err)
	}
	r1Active, err := r1Approved.Activate(now)
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	if err := fx.db.UpdateRelease(ctx, &r1Active); err != nil {
		t.Fatalf("update r1: %v", err)
	}

	// Activate r2 — supersedes r1
	r2Approved, err := r2.ApproveWith(approval.Approval{ReleaseID: "r2", Votes: []approval.Vote{
		{Identity: "bob", Decision: approval.Approve, Timestamp: now},
	}, UpdatedAt: now}, approval.MakerCheckerPolicy{RequiredApprovers: 1})
	if err != nil {
		t.Fatalf("approve r2: %v", err)
	}
	r2Active, err := r2Approved.Activate(now)
	if err != nil {
		t.Fatalf("activate r2: %v", err)
	}
	if err := fx.db.UpdateRelease(ctx, &r2Active); err != nil {
		t.Fatalf("update r2: %v", err)
	}
	r1Superseded, err := r1Active.Supersede("r2", now)
	if err != nil {
		t.Fatalf("supersede: %v", err)
	}
	if err := fx.db.UpdateRelease(ctx, &r1Superseded); err != nil {
		t.Fatalf("update r1 superseded: %v", err)
	}

	active, err := fx.db.ListActiveReleasesForEnvironment(ctx, release.EnvProd)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 1 || active[0].ID != "r2" {
		t.Fatalf("expected only r2 active in prod, got %+v", active)
	}

	got1, err := fx.db.GetRelease(ctx, "r1")
	if err != nil {
		t.Fatalf("get r1: %v", err)
	}
	if got1.Status != release.StatusSuperseded || got1.SupersededBy != "r2" {
		t.Fatalf("r1 = %+v want Superseded/SupersededBy=r2", got1.Status)
	}
}

func TestApprovalRoundTrip(t *testing.T) {
	fx := newReleaseFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()

	rel, _ := release.New(fx.capabilityID, 1, validManifest(), release.EnvDev, "alice")
	rel.ID = "r1"
	if err := fx.db.CreateRelease(ctx, &rel); err != nil {
		t.Fatalf("create release: %v", err)
	}

	a := &approval.Approval{ReleaseID: "r1", UpdatedAt: now}
	recorded, err := a.Record(approval.Vote{Identity: "bob", Decision: approval.Approve, Timestamp: now})
	if err != nil {
		t.Fatalf("record vote: %v", err)
	}
	if err := fx.db.CreateApproval(ctx, &recorded); err != nil {
		t.Fatalf("create approval: %v", err)
	}

	got, err := fx.db.GetApproval(ctx, "r1")
	if err != nil {
		t.Fatalf("get approval: %v", err)
	}
	if len(got.Votes) != 1 || got.Votes[0].Identity != "bob" {
		t.Fatalf("votes = %+v want one vote from bob", got.Votes)
	}

	// record another vote
	recorded2, err := got.Record(approval.Vote{Identity: "carol", Decision: approval.Approve, Timestamp: now})
	if err != nil {
		t.Fatalf("record second: %v", err)
	}
	if err := fx.db.UpdateApproval(ctx, &recorded2); err != nil {
		t.Fatalf("update approval: %v", err)
	}

	got2, err := fx.db.GetApproval(ctx, "r1")
	if err != nil {
		t.Fatalf("get approval 2: %v", err)
	}
	if len(got2.Votes) != 2 {
		t.Fatalf("votes = %d want 2", len(got2.Votes))
	}
}
