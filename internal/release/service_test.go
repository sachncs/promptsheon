package release_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/approval"
	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/harness"
	"github.com/sachncs/promptsheon/internal/release"
	"github.com/sachncs/promptsheon/internal/testdata"
	"github.com/sachncs/promptsheon/internal/testutil/harnessrepo"
)

// errMemStoreNotFound stands in for store.ErrNotFound in this
// test fixture. The release_test package cannot import
// internal/store (store imports harness via its Repository
// surface, and harness is reachable from release via its
// Service.WithHarness hook). Each precondition/approval/etc.
// store fixture defines its own sentinel and the test
// translates at the boundary.
var errMemStoreNotFound = errors.New("memstore: not found")

// memStore is a tiny in-memory Repository for service-level tests.
// SQLite-backed behaviour is covered in internal/store/sqlite_releases_test.go;
// here we exercise the Service's branching (Create, Vote, Activate,
// supersede, Rollback) against a fake.
//
// TEST-3: the harness-engineering aggregates (datasets,
// preconditions, eval runs, eval results) are delegated to the
// shared harnessrepo.MemRepo fixture. The release-side methods
// (releases, approvals) stay local because no other package
// needs them.
type memStore struct {
	releases  map[string]*release.Release
	approvals map[string]*approval.Approval

	// harnessRepo embeds the shared harness.Repository fixture
	// so duplicate boilerplate (CreateDataset, GetPrecondition,
	// etc.) doesn't recur in this file.
	harnessRepo *harnessrepo.MemRepo
}

func newMemStore() *memStore {
	return &memStore{
		releases:    make(map[string]*release.Release),
		approvals:   make(map[string]*approval.Approval),
		harnessRepo: harnessrepo.New(),
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
func (m *memStore) ActivateAtomic(_ context.Context, prior, next *release.Release) error {
	if prior != nil {
		if _, ok := m.releases[prior.ID]; !ok {
			return release.ErrNotFound
		}
		cp := *prior
		m.releases[prior.ID] = &cp
	}
	if _, ok := m.releases[next.ID]; !ok {
		return release.ErrNotFound
	}
	cp := *next
	m.releases[next.ID] = &cp
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

func validManifest() capability.Manifest { return testdata.NewManifest() }

// newServiceWithPolicy constructs a Service backed by the in-memory
// memStore and a caller-supplied approval policy. This replaces
// the PolicyKind-based constructor (DEAD-Rel-3): callers that
// want a Maker-Checker or Majority policy use the package-level
// helper functions.
func newService(t *testing.T, policy approval.Policy) (*release.Service, *memStore) {
	t.Helper()
	db := newMemStore()
	svc := release.NewService(db, db, policy)
	return svc, db
}

func TestServiceCreateVoteActivate(t *testing.T) {
	svc, _ := newService(t, approval.MakerCheckerPolicy{RequiredApprovers: 1})
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
	svc, _ := newService(t, approval.MakerCheckerPolicy{RequiredApprovers: 1})
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
	svc, _ := newService(t, approval.MakerCheckerPolicy{RequiredApprovers: 1})
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
	svc, _ := newService(t, approval.MakerCheckerPolicy{RequiredApprovers: 1})
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

func TestServiceActivateRunsPreconditions(t *testing.T) {
	t.Setenv("PROMPTSHEON_HARNESS_PRECONDITIONS", "true")
	svc, db := newService(t, approval.MakerCheckerPolicy{RequiredApprovers: 1})
	svc.WithHarness(harness.NewPreconditionRunner(), db)

	ctx := context.Background()
	now := time.Now().UTC()
	pre := &harness.Precondition{
		ID:           "p1",
		CapabilityID: "c1",
		Name:         "fail-loud",
		Command:      "exit 7",
		TimeoutSec:   5,
		Enabled:      true,
		CreatedAt:    now,
	}
	if err := db.CreatePrecondition(ctx, pre); err != nil {
		t.Fatalf("CreatePrecondition: %v", err)
	}

	r, err := svc.Create(ctx, "c1", 1, validManifest(), release.EnvProd, "alice")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Vote(ctx, r.ID, approval.Vote{Identity: "bob", Decision: approval.Approve}); err != nil {
		t.Fatalf("Vote: %v", err)
	}

	_, err = svc.Activate(ctx, r.ID)
	if err == nil {
		t.Fatal("expected Activate to fail when a precondition fails")
	}
	if !errors.Is(err, harness.ErrPreconditionFailed) {
		t.Fatalf("expected ErrPreconditionFailed, got %v", err)
	}
	var perr *harness.PreconditionError
	if !errors.As(err, &perr) {
		t.Fatalf("expected *harness.PreconditionError, got %T", err)
	}
	if len(perr.Failures) != 1 || perr.Failures[0].Name != "fail-loud" {
		t.Fatalf("unexpected failures: %+v", perr.Failures)
	}

	got, err := db.GetRelease(ctx, r.ID)
	if err != nil {
		t.Fatalf("GetRelease: %v", err)
	}
	if got.Status != release.StatusPending {
		t.Fatalf("release should remain Pending after precondition failure, got %s", got.Status)
	}
}

func TestServiceActivatePassesWhenAllPreconditionsPass(t *testing.T) {
	svc, db := newService(t, approval.MakerCheckerPolicy{RequiredApprovers: 1})
	svc.WithHarness(harness.NewPreconditionRunner(), db)

	ctx := context.Background()
	now := time.Now().UTC()
	if err := db.CreatePrecondition(ctx, &harness.Precondition{
		ID:           "p1",
		CapabilityID: "c1",
		Name:         "ok",
		Command:      "true",
		TimeoutSec:   5,
		Enabled:      true,
		CreatedAt:    now,
	}); err != nil {
		t.Fatalf("CreatePrecondition: %v", err)
	}

	r, err := svc.Create(ctx, "c1", 1, validManifest(), release.EnvProd, "alice")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Vote(ctx, r.ID, approval.Vote{Identity: "bob", Decision: approval.Approve}); err != nil {
		t.Fatalf("Vote: %v", err)
	}
	activated, err := svc.Activate(ctx, r.ID)
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if activated.Status != release.StatusActive {
		t.Fatalf("status = %s want active", activated.Status)
	}
}

func TestServiceClockSeam(t *testing.T) {
	svc, _ := newService(t, approval.MakerCheckerPolicy{RequiredApprovers: 1})
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

// ----- harness.Repository methods on memStore (delegated to harnessrepo) -----

func (m *memStore) CreateDataset(ctx context.Context, d *harness.Dataset) error {
	return m.harnessRepo.CreateDataset(ctx, d)
}
func (m *memStore) GetDataset(ctx context.Context, id string) (*harness.Dataset, error) {
	return m.harnessRepo.GetDataset(ctx, id)
}
func (m *memStore) ListDatasetsForCapability(ctx context.Context, capabilityID string) ([]*harness.Dataset, error) {
	return m.harnessRepo.ListDatasetsForCapability(ctx, capabilityID)
}
func (m *memStore) DeleteDataset(ctx context.Context, id string) error {
	return m.harnessRepo.DeleteDataset(ctx, id)
}
func (m *memStore) UpsertDatasetCases(ctx context.Context, datasetID string, cases []harness.DatasetCase) error {
	return m.harnessRepo.UpsertDatasetCases(ctx, datasetID, cases)
}
func (m *memStore) ListDatasetCases(ctx context.Context, datasetID string) ([]harness.DatasetCase, error) {
	return m.harnessRepo.ListDatasetCases(ctx, datasetID)
}
func (m *memStore) CreatePrecondition(ctx context.Context, p *harness.Precondition) error {
	return m.harnessRepo.CreatePrecondition(ctx, p)
}
func (m *memStore) ListPreconditionsForCapability(ctx context.Context, capabilityID string) ([]*harness.Precondition, error) {
	return m.harnessRepo.ListPreconditionsForCapability(ctx, capabilityID)
}
func (m *memStore) GetPrecondition(ctx context.Context, id string) (*harness.Precondition, error) {
	return m.harnessRepo.GetPrecondition(ctx, id)
}
func (m *memStore) UpdatePrecondition(ctx context.Context, p *harness.Precondition) error {
	return m.harnessRepo.UpdatePrecondition(ctx, p)
}
func (m *memStore) DeletePrecondition(ctx context.Context, id string) error {
	return m.harnessRepo.DeletePrecondition(ctx, id)
}
func (m *memStore) CreateEvalRun(ctx context.Context, r *harness.EvalRun) error {
	return m.harnessRepo.CreateEvalRun(ctx, r)
}
func (m *memStore) UpdateEvalRun(ctx context.Context, r *harness.EvalRun) error {
	return m.harnessRepo.UpdateEvalRun(ctx, r)
}
func (m *memStore) GetEvalRun(ctx context.Context, id string) (*harness.EvalRun, error) {
	return m.harnessRepo.GetEvalRun(ctx, id)
}
func (m *memStore) ListEvalRunsForRelease(ctx context.Context, releaseID string) ([]*harness.EvalRun, error) {
	return m.harnessRepo.ListEvalRunsForRelease(ctx, releaseID)
}
func (m *memStore) CreateEvalResults(ctx context.Context, results []harness.EvalResult) error {
	return m.harnessRepo.CreateEvalResults(ctx, results)
}
func (m *memStore) CreateEvalResult(ctx context.Context, r *harness.EvalResult) error {
	return m.harnessRepo.CreateEvalResult(ctx, r)
}
func (m *memStore) ListEvalResultsForRun(ctx context.Context, runID string) ([]harness.EvalResult, error) {
	return m.harnessRepo.ListEvalResultsForRun(ctx, runID)
}
