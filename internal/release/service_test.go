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
)

// memStore is a tiny in-memory Repository for service-level tests.
// SQLite-backed behaviour is covered in internal/store/sqlite_releases_test.go;
// here we exercise the Service's branching (Create, Vote, Activate,
// supersede, Rollback) against a fake.
type memStore struct {
	releases     map[string]*release.Release
	approvals    map[string]*approval.Approval
	preconds     map[string]*harness.Precondition
	datasets     map[string]*harness.Dataset
	cases        map[string][]harness.DatasetCase
	evalRuns     map[string]*harness.EvalRun
	evalResults  []harness.EvalResult
}

func newMemStore() *memStore {
	return &memStore{
		releases:  make(map[string]*release.Release),
		approvals: make(map[string]*approval.Approval),
		preconds:  make(map[string]*harness.Precondition),
		datasets:  make(map[string]*harness.Dataset),
		cases:     make(map[string][]harness.DatasetCase),
		evalRuns:  make(map[string]*harness.EvalRun),
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

func TestServiceActivateRunsPreconditions(t *testing.T) {
	svc, db := newService(t, release.PolicyMakerChecker, 1)
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
	svc, db := newService(t, release.PolicyMakerChecker, 1)
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

// ----- harness.Repository methods on memStore -----

func (m *memStore) CreateDataset(_ context.Context, d *harness.Dataset) error {
	m.datasets[d.ID] = d
	return nil
}
func (m *memStore) GetDataset(_ context.Context, id string) (*harness.Dataset, error) {
	if d, ok := m.datasets[id]; ok {
		return d, nil
	}
	return nil, errors.New("not found")
}
func (m *memStore) ListDatasetsForCapability(_ context.Context, capabilityID string) ([]*harness.Dataset, error) {
	var out []*harness.Dataset
	for _, d := range m.datasets {
		if d.CapabilityID == capabilityID {
			out = append(out, d)
		}
	}
	return out, nil
}
func (m *memStore) DeleteDataset(_ context.Context, id string) error {
	delete(m.datasets, id)
	return nil
}
func (m *memStore) UpsertDatasetCases(_ context.Context, datasetID string, cases []harness.DatasetCase) error {
	m.cases[datasetID] = cases
	return nil
}
func (m *memStore) ListDatasetCases(_ context.Context, datasetID string) ([]harness.DatasetCase, error) {
	return m.cases[datasetID], nil
}
func (m *memStore) CreatePrecondition(_ context.Context, p *harness.Precondition) error {
	m.preconds[p.ID] = p
	return nil
}
func (m *memStore) ListPreconditionsForCapability(_ context.Context, capabilityID string) ([]*harness.Precondition, error) {
	var out []*harness.Precondition
	for _, p := range m.preconds {
		if p.CapabilityID == capabilityID {
			out = append(out, p)
		}
	}
	return out, nil
}
func (m *memStore) DeletePrecondition(_ context.Context, id string) error {
	delete(m.preconds, id)
	return nil
}
func (m *memStore) CreateEvalRun(_ context.Context, r *harness.EvalRun) error {
	m.evalRuns[r.ID] = r
	return nil
}
func (m *memStore) UpdateEvalRun(_ context.Context, r *harness.EvalRun) error {
	m.evalRuns[r.ID] = r
	return nil
}
func (m *memStore) GetEvalRun(_ context.Context, id string) (*harness.EvalRun, error) {
	if r, ok := m.evalRuns[id]; ok {
		return r, nil
	}
	return nil, errors.New("not found")
}
func (m *memStore) ListEvalRunsForRelease(_ context.Context, releaseID string) ([]*harness.EvalRun, error) {
	var out []*harness.EvalRun
	for _, r := range m.evalRuns {
		if r.ReleaseID == releaseID {
			out = append(out, r)
		}
	}
	return out, nil
}
func (m *memStore) CreateEvalResults(_ context.Context, results []harness.EvalResult) error {
	m.evalResults = append(m.evalResults, results...)
	return nil
}
func (m *memStore) ListEvalResultsForRun(_ context.Context, runID string) ([]harness.EvalResult, error) {
	var out []harness.EvalResult
	for _, r := range m.evalResults {
		if r.RunID == runID {
			out = append(out, r)
		}
	}
	return out, nil
}
