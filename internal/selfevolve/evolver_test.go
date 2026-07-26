package selfevolve

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/harness"
	"github.com/sachncs/promptsheon/internal/store"
)

// fakeRepo is an in-memory implementation of the evolver's
// Repository. It supports the minimum surface needed for the
// evolver to run a full cycle: capabilities with a
// SelfEvolveConfig, an active release per (cap, env), a
// dataset with cases, and eval runs with results. It is the
// only test double the evolver's test suite uses — every
// collaborator is faked so the test is hermetic.
type fakeRepo struct {
	mu sync.Mutex

	capabilities   map[string]*capability.Capability
	versions       map[string]*capability.Version
	versionsByCap  map[string][]*capability.Version // capabilityID → all versions, sorted by Version desc
	releases       map[string]*ReleaseRecord
	activeReleases map[string]string // capabilityID+env → releaseID
	datasets       map[string]*harness.Dataset
	cases          map[string][]harness.DatasetCase // datasetID → cases
	evalRuns       map[string]*harness.EvalRun
	evalResults    map[string][]harness.EvalResult   // runID → results
	state          map[string]*store.SelfEvolveState // capabilityID+env → state
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		capabilities:   map[string]*capability.Capability{},
		versions:       map[string]*capability.Version{},
		versionsByCap:  map[string][]*capability.Version{},
		releases:       map[string]*ReleaseRecord{},
		activeReleases: map[string]string{},
		datasets:       map[string]*harness.Dataset{},
		cases:          map[string][]harness.DatasetCase{},
		evalRuns:       map[string]*harness.EvalRun{},
		evalResults:    map[string][]harness.EvalResult{},
		state:          map[string]*store.SelfEvolveState{},
	}
}

func stateKey(capID, env string) string { return capID + "|" + env }

func (r *fakeRepo) seedCapability(capID, datasetID string, env string, promptText string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	promptHash := hashString(promptText)
	modelHash := hashString("model-policy:" + capID)
	rtHash := hashString("runtime-policy:" + capID)
	manifest := capability.Manifest{
		Prompt:        capability.ArtifactRef{Kind: capability.ArtifactPrompt, Hash: promptHash},
		ModelPolicy:   capability.ArtifactRef{Kind: capability.ArtifactModelPolicy, Hash: modelHash},
		RuntimePolicy: capability.ArtifactRef{Kind: capability.ArtifactRuntimePolicy, Hash: rtHash},
		Context:       capability.ArtifactRef{Kind: capability.ArtifactContext, Hash: rtHash},
		Memory:        capability.ArtifactRef{Kind: capability.ArtifactMemory, Hash: rtHash},
	}
	mh, _ := capability.ComputeManifestHash(manifest)
	v := &capability.Version{
		ID:           "v-1-" + capID,
		CapabilityID: capID,
		Version:      1,
		Manifest:     manifest,
		ManifestHash: mh,
		CreatedAt:    time.Now().UTC(),
		CreatedBy:    "seed",
	}
	r.versions[v.ID] = v
	r.versionsByCap[capID] = append(r.versionsByCap[capID], v)
	sort.Slice(r.versionsByCap[capID], func(i, j int) bool {
		return r.versionsByCap[capID][i].Version > r.versionsByCap[capID][j].Version
	})
	rel := &ReleaseRecord{
		ID:                "rel-active-" + capID,
		CapabilityID:      capID,
		CapabilityVersion: 1,
		Manifest:          manifest,
		Environment:       env,
		Status:            "active",
		CreatedBy:         "seed",
		CreatedAt:         time.Now().UTC(),
	}
	r.releases[rel.ID] = rel
	r.activeReleases[stateKey(capID, env)] = rel.ID
	cap := &capability.Capability{
		ID:          capID,
		ProjectID:   "p1",
		Name:        "fake",
		Description: "fake cap",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
		SelfEvolve: capability.SelfEvolveConfig{
			Enabled:      true,
			MinScore:     0.9,
			MaxRevisions: 10,
			CooldownSec:  15,
			TargetEnv:    env,
			DatasetID:    datasetID,
		},
	}
	r.capabilities[capID] = cap
}

func (r *fakeRepo) seedDataset(datasetID string, cases []harness.DatasetCase) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.datasets[datasetID] = &harness.Dataset{ID: datasetID, Name: datasetID}
	r.cases[datasetID] = cases
}

// recordEvalRun inserts a finished eval run with the supplied
// results. The evolver's RunOnce reads this as the "last
// eval" for the active release.
func (r *fakeRepo) recordEvalRun(releaseID string, passed, failed int, withErrors bool) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := "erun-" + releaseID + "-" + fmt.Sprint(len(r.evalRuns))
	total := passed + failed
	var score float64
	if total > 0 {
		score = float64(passed) / float64(total)
	}
	run := &harness.EvalRun{
		ID:        id,
		ReleaseID: releaseID,
		DatasetID: r.findDatasetIDForRelease(releaseID),
		Scorer:    "contains",
		Score:     score,
		Passed:    passed,
		Failed:    failed,
		Total:     total,
		Status:    harness.RunFailed,
		StartedAt: time.Now().UTC(),
	}
	if failed == 0 {
		run.Status = harness.RunPassed
	}
	r.evalRuns[id] = run
	var results []harness.EvalResult
	for i := 0; i < passed; i++ {
		results = append(results, harness.EvalResult{
			ID:     id + "-r" + fmt.Sprint(i),
			RunID:  id,
			CaseID: "case-" + fmt.Sprint(i),
			Seq:    i,
			Passed: true,
			Actual: json.RawMessage(`"hello"`),
		})
	}
	for i := 0; i < failed; i++ {
		seq := passed + i
		errStr := ""
		if withErrors {
			errStr = "provider error"
		}
		results = append(results, harness.EvalResult{
			ID:     id + "-r" + fmt.Sprint(seq),
			RunID:  id,
			CaseID: "case-" + fmt.Sprint(seq),
			Seq:    seq,
			Passed: false,
			Actual: json.RawMessage(`"wrong"`),
			Error:  errStr,
		})
	}
	r.evalResults[id] = results
	return id
}

func (r *fakeRepo) findDatasetIDForRelease(releaseID string) string {
	for _, rel := range r.releases {
		if rel.ID == releaseID {
			// The seed code sets the dataset via
			// the cap; we don't store it on the
			// release. Walk back via capability.
			for _, c := range r.capabilities {
				if c.ID == rel.CapabilityID && c.SelfEvolve.DatasetID != "" {
					return c.SelfEvolve.DatasetID
				}
			}
		}
	}
	return ""
}

// --- Repository interface ---

func (r *fakeRepo) GetCapability(ctx context.Context, id string) (*capability.Capability, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.capabilities[id]
	if !ok {
		return nil, fmt.Errorf("fakeRepo: capability not found: %s", id)
	}
	cp := *c
	return &cp, nil
}
func (r *fakeRepo) GetVersion(ctx context.Context, id string) (*capability.Version, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.versions[id]
	if !ok {
		return nil, fmt.Errorf("fakeRepo: version not found: %s", id)
	}
	cp := *v
	return &cp, nil
}
func (r *fakeRepo) GetVersionByNumber(ctx context.Context, capabilityID string, version int) (*capability.Version, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range r.versionsByCap[capabilityID] {
		if v.Version == version {
			cp := *v
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("fakeRepo: version %d not found for %s", version, capabilityID)
}
func (r *fakeRepo) CreateVersion(ctx context.Context, v *capability.Version) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.versions[v.ID]; ok {
		return fmt.Errorf("fakeRepo: version id %s exists", v.ID)
	}
	r.versions[v.ID] = v
	r.versionsByCap[v.CapabilityID] = append(r.versionsByCap[v.CapabilityID], v)
	sort.Slice(r.versionsByCap[v.CapabilityID], func(i, j int) bool {
		return r.versionsByCap[v.CapabilityID][i].Version > r.versionsByCap[v.CapabilityID][j].Version
	})
	return nil
}
func (r *fakeRepo) UpdateSelfEvolveConfig(ctx context.Context, id string, cfg capability.SelfEvolveConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.capabilities[id]
	if !ok {
		return fmt.Errorf("fakeRepo: cap not found")
	}
	c.SelfEvolve = cfg
	r.capabilities[id] = c
	return nil
}
func (r *fakeRepo) LoadSelfEvolveState(ctx context.Context, capabilityID, targetEnv string) (*store.SelfEvolveState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state[stateKey(capabilityID, targetEnv)], nil
}
func (r *fakeRepo) SaveSelfEvolveState(ctx context.Context, st *store.SelfEvolveState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state[stateKey(st.CapabilityID, st.TargetEnv)] = st
	return nil
}

// --- harness.Repository ---
func (r *fakeRepo) CreateDataset(ctx context.Context, d *harness.Dataset) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.datasets[d.ID] = d
	return nil
}
func (r *fakeRepo) GetDataset(ctx context.Context, id string) (*harness.Dataset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.datasets[id]
	if !ok {
		return nil, fmt.Errorf("dataset not found")
	}
	cp := *d
	return &cp, nil
}
func (r *fakeRepo) ListDatasetsForCapability(ctx context.Context, capabilityID string) ([]*harness.Dataset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Single dataset per cap in the test.
	for _, d := range r.datasets {
		cp := *d
		return []*harness.Dataset{&cp}, nil
	}
	return nil, nil
}
func (r *fakeRepo) DeleteDataset(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.datasets, id)
	return nil
}
func (r *fakeRepo) UpsertDatasetCases(ctx context.Context, datasetID string, cases []harness.DatasetCase) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cases[datasetID] = cases
	return nil
}
func (r *fakeRepo) ListDatasetCases(ctx context.Context, datasetID string) ([]harness.DatasetCase, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cases[datasetID], nil
}
func (r *fakeRepo) CreatePrecondition(ctx context.Context, p *harness.Precondition) error {
	return nil
}
func (r *fakeRepo) GetPrecondition(ctx context.Context, id string) (*harness.Precondition, error) {
	return nil, nil
}
func (r *fakeRepo) ListPreconditionsForCapability(ctx context.Context, capabilityID string) ([]*harness.Precondition, error) {
	return nil, nil
}
func (r *fakeRepo) UpdatePrecondition(ctx context.Context, p *harness.Precondition) error {
	return nil
}
func (r *fakeRepo) DeletePrecondition(ctx context.Context, id string) error {
	return nil
}
func (r *fakeRepo) CreateEvalRun(ctx context.Context, run *harness.EvalRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.evalRuns[run.ID] = run
	return nil
}
func (r *fakeRepo) UpdateEvalRun(ctx context.Context, run *harness.EvalRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.evalRuns[run.ID] = run
	return nil
}
func (r *fakeRepo) GetEvalRun(ctx context.Context, id string) (*harness.EvalRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.evalRuns[id]
	if !ok {
		return nil, fmt.Errorf("eval run not found")
	}
	cp := *run
	return &cp, nil
}
func (r *fakeRepo) ListEvalRunsForRelease(ctx context.Context, releaseID string) ([]*harness.EvalRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*harness.EvalRun
	for _, run := range r.evalRuns {
		if run.ReleaseID == releaseID {
			cp := *run
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (r *fakeRepo) GetActiveReleaseID(ctx context.Context, capabilityID string) (string, error) {
	return "", nil
}
func (r *fakeRepo) CreateEvalResults(ctx context.Context, results []harness.EvalResult) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, rr := range results {
		existing := r.evalResults[rr.RunID]
		// Replace any with the same id.
		replaced := false
		for i, e := range existing {
			if e.ID == rr.ID {
				existing[i] = rr
				replaced = true
				break
			}
		}
		if !replaced {
			existing = append(existing, rr)
		}
		r.evalResults[rr.RunID] = existing
	}
	return nil
}
func (r *fakeRepo) CreateEvalResult(ctx context.Context, result *harness.EvalResult) error {
	return r.CreateEvalResults(ctx, []harness.EvalResult{*result})
}
func (r *fakeRepo) ListEvalResultsForRun(ctx context.Context, runID string) ([]harness.EvalResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.evalResults[runID], nil
}

// --- evolver-specific extensions (ActiveReleaseID per env,
// GetRelease, LastEvalRun, CreateRelease, UpdateReleaseStatus,
// SelfEvolveState load/save in evolver's store-blob shape) ---

func (r *fakeRepo) ActiveReleaseID(ctx context.Context, capabilityID, env string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.activeReleases[stateKey(capabilityID, env)], nil
}
func (r *fakeRepo) GetRelease(ctx context.Context, id string) (*ReleaseRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rel, ok := r.releases[id]
	if !ok {
		return nil, fmt.Errorf("release not found")
	}
	cp := *rel
	return &cp, nil
}
func (r *fakeRepo) LastEvalRun(ctx context.Context, releaseID string) (*harness.EvalRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var last *harness.EvalRun
	for _, run := range r.evalRuns {
		if run.ReleaseID == releaseID {
			if last == nil || run.StartedAt.After(last.StartedAt) {
				last = run
			}
		}
	}
	if last == nil {
		return nil, nil
	}
	cp := *last
	return &cp, nil
}
func (r *fakeRepo) UpdateReleaseStatus(ctx context.Context, releaseID, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	rel, ok := r.releases[releaseID]
	if !ok {
		return fmt.Errorf("release not found")
	}
	rel.Status = status
	r.releases[releaseID] = rel
	return nil
}
func (r *fakeRepo) CreateRelease(ctx context.Context, rel ReleaseRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.releases[rel.ID] = &rel
	if rel.Status == "active" {
		// Supersede any prior active in the same (cap, env).
		for k, rid := range r.activeReleases {
			if k == stateKey(rel.CapabilityID, rel.Environment) {
				if prior, ok := r.releases[rid]; ok {
					prior.Status = "superseded"
					r.releases[rid] = prior
				}
			}
		}
		r.activeReleases[stateKey(rel.CapabilityID, rel.Environment)] = rel.ID
	}
	return nil
}
func (r *fakeRepo) SelfEvolveStateGet(capID, env string) *store.SelfEvolveState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state[stateKey(capID, env)]
}

// --- helpers for tests ---

func hashString(s string) string {
	const hex = "0123456789abcdef"
	out := make([]byte, 64)
	for i := 0; i < 32; i++ {
		out[2*i] = hex[(len(s)+i)%16]
		out[2*i+1] = hex[(len(s)+i*3)%16]
	}
	return string(out)
}

// activeReleaseState returns the current active release id
// for a (cap, env). Used in assertions.
func (r *fakeRepo) activeReleaseID(capID, env string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.activeReleases[stateKey(capID, env)]
}

// releaseByID returns a copy of the release record. Used
// in assertions.
func (r *fakeRepo) releaseByID(id string) *ReleaseRecord {
	r.mu.Lock()
	defer r.mu.Unlock()
	rel, ok := r.releases[id]
	if !ok {
		return nil
	}
	cp := *rel
	return &cp
}

// versionByNumber returns a copy of the version record.
func (r *fakeRepo) versionByNumber(capID string, n int) *capability.Version {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range r.versionsByCap[capID] {
		if v.Version == n {
			cp := *v
			return &cp
		}
	}
	return nil
}

// fakeLoader is a CAS-free in-memory prompt store. Tests can
// pre-load prompts; the evolver's WritePrompt will be
// captured by the loader.
type fakeLoader struct {
	mu      sync.Mutex
	blobs   map[string]string
	writeCh chan string
}

func newFakeLoader() *fakeLoader {
	return &fakeLoader{blobs: map[string]string{}, writeCh: make(chan string, 8)}
}

func (l *fakeLoader) seed(hash, text string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.blobs[hash] = text
}

func (l *fakeLoader) LoadPrompt(ctx context.Context, hash string) ([]byte, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.blobs[hash]
	if !ok {
		return nil, fmt.Errorf("fakeLoader: not found: %s", hash)
	}
	return []byte(t), nil
}

func (l *fakeLoader) WritePrompt(ctx context.Context, text string) (string, error) {
	h := hashString(text)
	l.mu.Lock()
	l.blobs[h] = text
	l.mu.Unlock()
	select {
	case l.writeCh <- h:
	default:
	}
	return h, nil
}

// fakeRevisionLLM is a programmable RevisionLLM. Tests set
// newPrompt on the first call; the evolver reads it.
type fakeRevisionLLM struct {
	mu        sync.Mutex
	newPrompt string
	rationale string
	err       error
	calls     int
}

func (r *fakeRevisionLLM) Revise(ctx context.Context, req ReviseRequest) (*ReviseResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	return &ReviseResponse{NewPrompt: r.newPrompt, Rationale: r.rationale}, nil
}

// fakeActivator is a programmable ReleaseActivator. The
// fakeActivator does the same bookkeeping the real
// release.Service.SelfActivate does: mark the release
// active and supersede the prior active in the same env.
type fakeActivator struct {
	mu       sync.Mutex
	calls    int
	failNext bool
	failErr  error
	repo     *fakeRepo
}

func (a *fakeActivator) SelfActivate(ctx context.Context, releaseID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls++
	if a.failNext {
		a.failNext = false
		return a.failErr
	}
	if a.repo == nil {
		return nil
	}
	a.repo.mu.Lock()
	defer a.repo.mu.Unlock()
	rel, ok := a.repo.releases[releaseID]
	if !ok {
		return fmt.Errorf("fakeActivator: release %s not found", releaseID)
	}
	// Mark the prior active as superseded.
	for k, rid := range a.repo.activeReleases {
		if rid != releaseID {
			capID, env, _ := splitStateKey(k)
			if capID == rel.CapabilityID && env == rel.Environment {
				if prior, ok := a.repo.releases[rid]; ok {
					prior.Status = "superseded"
					a.repo.releases[rid] = prior
				}
			}
		}
	}
	rel.Status = "active"
	a.repo.releases[releaseID] = rel
	a.repo.activeReleases[stateKey(rel.CapabilityID, rel.Environment)] = releaseID
	return nil
}

func splitStateKey(k string) (capID, env, rest string) {
	// stateKey is capID + "|" + env. The env can contain
	// '|' if it ever does, but in practice it doesn't.
	for i := 0; i < len(k); i++ {
		if k[i] == '|' {
			return k[:i], k[i+1:], ""
		}
	}
	return k, "", ""
}

// fakeAuditor captures audit rows.
type fakeAuditor struct {
	mu   sync.Mutex
	rows []auditRow
}
type auditRow struct {
	Action string
	Target string
	Detail map[string]any
}

func (a *fakeAuditor) Audit(ctx context.Context, action, target string, detail map[string]any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rows = append(a.rows, auditRow{action, target, detail})
}

// fakeValidator is a programmable Validator. The score
// field is returned in the EvalRun on Validate.
type fakeValidator struct {
	mu    sync.Mutex
	score float64
	err   error
}

func (v *fakeValidator) Validate(ctx context.Context, capabilityID string, promptBytes []byte, datasetID string) (*harness.EvalRun, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.err != nil {
		return nil, v.err
	}
	return &harness.EvalRun{
		ID:        "validation-" + capabilityID,
		ReleaseID: "self-evolve-validate-" + capabilityID,
		DatasetID: datasetID,
		Scorer:    "contains",
		Score:     v.score,
		Passed:    int(v.score * 3),
		Failed:    3 - int(v.score*3),
		Total:     3,
		Status:    harness.RunPassed,
		StartedAt: time.Now().UTC(),
	}, nil
}

// Tests below.
func TestEvolver_RunOnce_DisabledSkips(t *testing.T) {
	inner := newFakeRepo()
	inner.seedCapability("c1", "ds1", "dev", "old prompt")
	repo := inner
	inner.capabilities["c1"].SelfEvolve.Enabled = false
	loader := newFakeLoader()
	ev := NewEvolver(repo, loader, &fakeRevisionLLM{}, &fakeValidator{}, mustNewPromoter(t, repo, loader, &fakeActivator{repo: inner}, &fakeAuditor{}), &fakeAuditor{}, nil)
	res, err := ev.RunOnce(context.Background(), "c1")
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !res.Skipped {
		t.Errorf("expected skipped, got %+v", res)
	}
}

func TestEvolver_RunOnce_AboveThresholdSkips(t *testing.T) {
	inner := newFakeRepo()
	inner.seedCapability("c1", "ds1", "dev", "old prompt")
	inner.seedDataset("ds1", []harness.DatasetCase{{ID: "c1", DatasetID: "ds1", Seq: 0, Inputs: json.RawMessage(`{}`), Expected: json.RawMessage(`"x"`)}})
	rel := inner.activeReleaseID("c1", "dev")
	inner.recordEvalRun(rel, 3, 0, false) // 3/3 = 1.0
	repo := inner
	loader := newFakeLoader()
	loader.seed(inner.versionsByCap["c1"][0].Manifest.Prompt.Hash, "old prompt")
	ev := NewEvolver(repo, loader, &fakeRevisionLLM{}, &fakeValidator{score: 1.0}, mustNewPromoter(t, repo, loader, &fakeActivator{repo: inner}, &fakeAuditor{}), &fakeAuditor{}, nil)
	res, err := ev.RunOnce(context.Background(), "c1")
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !res.Skipped {
		t.Errorf("expected skipped, got %+v", res)
	}
}

func TestEvolver_RunOnce_BelowThresholdPromotes(t *testing.T) {
	inner := newFakeRepo()
	inner.seedCapability("c1", "ds1", "dev", "old prompt")
	inner.seedDataset("ds1", []harness.DatasetCase{
		{ID: "k0", DatasetID: "ds1", Seq: 0, Inputs: json.RawMessage(`{"q":"ping"}`), Expected: json.RawMessage(`"pong"`)},
		{ID: "k1", DatasetID: "ds1", Seq: 1, Inputs: json.RawMessage(`{"q":"hi"}`), Expected: json.RawMessage(`"pong"`)},
		{ID: "k2", DatasetID: "ds1", Seq: 2, Inputs: json.RawMessage(`{"q":"yo"}`), Expected: json.RawMessage(`"pong"`)},
	})
	rel := inner.activeReleaseID("c1", "dev")
	// Pre-seed the loader with the active prompt.
	loader := newFakeLoader()
	promptHash := inner.versionsByCap["c1"][0].Manifest.Prompt.Hash
	loader.seed(promptHash, "old prompt")
	// Failed eval run: 0/3.
	inner.recordEvalRun(rel, 0, 3, false)
	repo := inner
	revision := &fakeRevisionLLM{newPrompt: "new better prompt"}
	validator := &fakeValidator{score: 1.0} // validation passes
	activator := &fakeActivator{repo: inner}
	auditor := &fakeAuditor{}
	ev := NewEvolver(repo, loader, revision, validator, mustNewPromoter(t, repo, loader, activator, auditor), auditor, nil)
	res, err := ev.RunOnce(context.Background(), "c1")
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !res.Promoted {
		t.Errorf("expected promoted, got %+v", res)
	}
	if res.Revisions != 1 {
		t.Errorf("expected revisions=1, got %d", res.Revisions)
	}
	// The new active release must be a different id.
	newRelID := inner.activeReleaseID("c1", "dev")
	if newRelID == rel {
		t.Errorf("expected active release to change, still %s", rel)
	}
	// The new release's version must be 2.
	newRel := inner.releaseByID(newRelID)
	if newRel.CapabilityVersion != 2 {
		t.Errorf("expected new version 2, got %d", newRel.CapabilityVersion)
	}
	// The new prompt must be in the loader.
	if _, err := loader.LoadPrompt(context.Background(), newRel.Manifest.Prompt.Hash); err != nil {
		t.Errorf("expected new prompt in CAS, got %v", err)
	}
	// SelfActivate must have been called exactly once.
	if activator.calls != 1 {
		t.Errorf("expected 1 SelfActivate call, got %d", activator.calls)
	}
	// Audit rows: detect + revise + validate + promote.
	want := map[string]bool{AuditDetect: false, AuditRevise: false, AuditValidate: false, AuditPromote: false}
	for _, r := range auditor.rows {
		if _, ok := want[r.Action]; ok {
			want[r.Action] = true
		}
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("expected audit action %q, not present", k)
		}
	}
}

func TestEvolver_RunOnce_ValidateBelowThresholdRetries(t *testing.T) {
	inner := newFakeRepo()
	inner.seedCapability("c1", "ds1", "dev", "old prompt")
	inner.seedDataset("ds1", []harness.DatasetCase{
		{ID: "k0", DatasetID: "ds1", Seq: 0, Inputs: json.RawMessage(`{}`), Expected: json.RawMessage(`"x"`)},
	})
	rel := inner.activeReleaseID("c1", "dev")
	loader := newFakeLoader()
	loader.seed(inner.versionsByCap["c1"][0].Manifest.Prompt.Hash, "old prompt")
	inner.recordEvalRun(rel, 0, 1, false)
	// Set MaxRevisions to 3 so we don't hit the cap.
	inner.capabilities["c1"].SelfEvolve.MaxRevisions = 3
	repo := inner
	revision := &fakeRevisionLLM{newPrompt: "v2 prompt"}
	validator := &fakeValidator{score: 0.5} // below threshold
	activator := &fakeActivator{repo: inner}
	auditor := &fakeAuditor{}
	ev := NewEvolver(repo, loader, revision, validator, mustNewPromoter(t, repo, loader, activator, auditor), auditor, nil)
	res, err := ev.RunOnce(context.Background(), "c1")
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if res.Promoted {
		t.Errorf("expected NOT promoted, got %+v", res)
	}
	if revision.calls != 3 {
		t.Errorf("expected 3 revision attempts, got %d", revision.calls)
	}
	if activator.calls != 0 {
		t.Errorf("expected 0 SelfActivate calls, got %d", activator.calls)
	}
	if res.RejectReason == "" {
		t.Errorf("expected non-empty RejectReason")
	}
}

func TestEvolver_RunOnce_RevisionLLMErrorContinues(t *testing.T) {
	inner := newFakeRepo()
	inner.seedCapability("c1", "ds1", "dev", "old prompt")
	inner.seedDataset("ds1", []harness.DatasetCase{
		{ID: "k0", DatasetID: "ds1", Seq: 0, Inputs: json.RawMessage(`{}`), Expected: json.RawMessage(`"x"`)},
	})
	rel := inner.activeReleaseID("c1", "dev")
	loader := newFakeLoader()
	loader.seed(inner.versionsByCap["c1"][0].Manifest.Prompt.Hash, "old prompt")
	inner.recordEvalRun(rel, 0, 1, false)
	inner.capabilities["c1"].SelfEvolve.MaxRevisions = 3
	repo := inner
	revision := &fakeRevisionLLM{err: fmt.Errorf("synthetic revise error")}
	validator := &fakeValidator{score: 1.0}
	activator := &fakeActivator{repo: inner}
	auditor := &fakeAuditor{}
	ev := NewEvolver(repo, loader, revision, validator, mustNewPromoter(t, repo, loader, activator, auditor), auditor, nil)
	res, err := ev.RunOnce(context.Background(), "c1")
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if res.Promoted {
		t.Errorf("expected NOT promoted (LLM always errored), got %+v", res)
	}
	if revision.calls != 3 {
		t.Errorf("expected 3 revision attempts, got %d", revision.calls)
	}
}

func TestEvolver_RunOnce_CooldownSkips(t *testing.T) {
	inner := newFakeRepo()
	inner.seedCapability("c1", "ds1", "dev", "old prompt")
	inner.seedDataset("ds1", []harness.DatasetCase{
		{ID: "k0", DatasetID: "ds1", Seq: 0, Inputs: json.RawMessage(`{}`), Expected: json.RawMessage(`"x"`)},
	})
	rel := inner.activeReleaseID("c1", "dev")
	loader := newFakeLoader()
	loader.seed(inner.versionsByCap["c1"][0].Manifest.Prompt.Hash, "old prompt")
	inner.recordEvalRun(rel, 0, 1, false)
	// Set a recent LastPromoteAt to trip cooldown.
	inner.capabilities["c1"].SelfEvolve.CooldownSec = 900
	repo := inner
	now := time.Now().UTC()
	inner.state[stateKey("c1", "dev")] = &store.SelfEvolveState{
		CapabilityID: "c1", TargetEnv: "dev",
		LastPromoteAt: &now, LastStatus: "promoted",
	}
	revision := &fakeRevisionLLM{newPrompt: "x"}
	validator := &fakeValidator{score: 1.0}
	ev := NewEvolver(repo, loader, revision, validator, mustNewPromoter(t, repo, loader, &fakeActivator{repo: inner}, &fakeAuditor{}), &fakeAuditor{}, nil)
	res, err := ev.RunOnce(context.Background(), "c1")
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !res.Skipped {
		t.Errorf("expected skipped due to cooldown, got %+v", res)
	}
	if revision.calls != 0 {
		t.Errorf("expected 0 revision calls during cooldown, got %d", revision.calls)
	}
}

func TestEvolver_RunOnce_NoActiveRelease(t *testing.T) {
	inner := newFakeRepo()
	inner.seedCapability("c1", "ds1", "dev", "old prompt")
	// Seed a release so the "no eval run yet" path doesn't
	// fire first, then delete it so the active lookup
	// returns empty.
	rel := inner.activeReleaseID("c1", "dev")
	inner.recordEvalRun(rel, 0, 1, false)
	delete(inner.activeReleases, stateKey("c1", "dev"))
	repo := inner
	loader := newFakeLoader()
	activator := &fakeActivator{repo: inner}
	ev := NewEvolver(repo, loader, &fakeRevisionLLM{}, &fakeValidator{}, mustNewPromoter(t, repo, loader, activator, &fakeAuditor{}), &fakeAuditor{}, nil)
	res, err := ev.RunOnce(context.Background(), "c1")
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !res.Skipped || res.RejectReason != "no active release" {
		t.Errorf("expected skipped/no active release, got %+v", res)
	}
}

func TestEvolver_RunOnce_EmptyDatasetID(t *testing.T) {
	inner := newFakeRepo()
	inner.seedCapability("c1", "", "dev", "old prompt")
	repo := inner
	loader := newFakeLoader()
	ev := NewEvolver(repo, loader, &fakeRevisionLLM{}, &fakeValidator{}, mustNewPromoter(t, repo, loader, &fakeActivator{repo: inner}, &fakeAuditor{}), &fakeAuditor{}, nil)
	_, err := ev.RunOnce(context.Background(), "c1")
	if err == nil {
		t.Errorf("expected error for empty DatasetID")
	}
}

func TestEvolver_LLMRevisionStrategy_RejectsEmpty(t *testing.T) {
	called := 0
	invoke := func(ctx context.Context, req LLMInvokeRequest) (string, error) {
		called++
		return "", nil
	}
	s := NewLLMRevisionStrategy(invoke)
	_, err := s.Revise(context.Background(), ReviseRequest{CurrentPrompt: "old"})
	if err == nil {
		t.Fatalf("expected error on empty LLM output, got nil")
	}
	if called != 1 {
		t.Errorf("expected invoke called once, got %d", called)
	}
}

func TestEvolver_LLMRevisionStrategy_HappyPath(t *testing.T) {
	invoke := func(ctx context.Context, req LLMInvokeRequest) (string, error) {
		if req.System != DefaultRevisionLLMSystem {
			t.Errorf("system prompt mismatch")
		}
		return "revised prompt", nil
	}
	s := NewLLMRevisionStrategy(invoke)
	resp, err := s.Revise(context.Background(), ReviseRequest{CurrentPrompt: "old", FailingCases: []FailingCase{{Seq: 0, Expected: "x", Actual: "y"}}})
	if err != nil {
		t.Fatalf("Revise: %v", err)
	}
	if resp.NewPrompt != "revised prompt" {
		t.Errorf("unexpected prompt: %q", resp.NewPrompt)
	}
}

func TestEvolver_Promoter_Promote(t *testing.T) {
	inner := newFakeRepo()
	inner.seedCapability("c1", "ds1", "dev", "old prompt")
	inner.seedDataset("ds1", []harness.DatasetCase{{ID: "k0", DatasetID: "ds1", Seq: 0, Inputs: json.RawMessage(`{}`), Expected: json.RawMessage(`"x"`)}})
	oldRelID := inner.activeReleaseID("c1", "dev")
	loader := newFakeLoader()
	loader.seed(inner.versionsByCap["c1"][0].Manifest.Prompt.Hash, "old prompt")
	repo := inner
	activator := &fakeActivator{repo: inner}
	auditor := &fakeAuditor{}
	p := mustNewPromoter(t, repo, loader, activator, auditor)
	res, err := p.Promote(context.Background(), "c1", "dev", oldRelID, "new prompt text")
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if res.NewVersionID == "" {
		t.Errorf("expected new version id")
	}
	if res.NewReleaseID == "" {
		t.Errorf("expected new release id")
	}
	if res.NewReleaseID == oldRelID {
		t.Errorf("expected a new release id, got the old one")
	}
	// Old release must be superseded.
	old := inner.releaseByID(oldRelID)
	if old.Status != "superseded" {
		t.Errorf("expected old release to be superseded, got %q", old.Status)
	}
	// New release must be active.
	if inner.activeReleaseID("c1", "dev") != res.NewReleaseID {
		t.Errorf("expected new release active")
	}
	if activator.calls != 1 {
		t.Errorf("expected 1 SelfActivate call, got %d", activator.calls)
	}
}

// silent unused-import guards for tools the linter sees as
// referenced only by the test build; the imports stay live
// for future tests.

// mustNewPromoter constructs a Promoter for tests and fails the
// test fast if any required dependency is missing. The previous
// single-return NewPromoter silently dropped nil-dependency
// errors; the constructor now returns (*Promoter, error) and
// tests use this helper to keep call sites compact.
func mustNewPromoter(t *testing.T, repo Repository, loader PromptLoader, activator ReleaseActivator, auditor Auditor) *Promoter {
	t.Helper()
	p, err := NewPromoter(repo, loader, activator, auditor)
	if err != nil {
		t.Fatalf("NewPromoter: %v", err)
	}
	return p
}
