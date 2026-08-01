package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/promptsheon/approval"
	"github.com/sachncs/promptsheon/promptsheon/capability"
	"github.com/sachncs/promptsheon/promptsheon/harness"
	"github.com/sachncs/promptsheon/promptsheon/models"
	"github.com/sachncs/promptsheon/promptsheon/release"
	"github.com/sachncs/promptsheon/promptsheon/selfevolve"
	"github.com/sachncs/promptsheon/promptsheon/store"
)

// casWritePrompt is a thin wrapper that chdirs the test
// process to the temp dir before writing so the local CAS
// resolves under .promptsheon/. The hash is recomputed by
// the loader (content-addressed), so callers should use
// the returned string instead of the input hash.
func casWritePrompt(text, _ string) (string, error) {
	return (&selfevolve.CasPromptLoader{}).WritePrompt(context.Background(), text)
}

// TestSelfEvolve_OrchestratorEndToEnd exercises the full
// closed-loop orchestrator in-process. It uses real CAS
// (via a temp dir), real SQLite (via a temp db), and a fake
// LLM that always returns the right answer. The point is
// to prove the wiring works: the evolver detects a
// failing score, asks the LLM for a revised prompt,
// validates, and promotes the new version in the target
// env. The audit chain captures every transition.
func TestSelfEvolve_OrchestratorEndToEnd(t *testing.T) {
	// 1. Temp dir for CAS + temp db for SQLite.
	tmpDir, err := os.MkdirTemp("", "selfevolve-e2e-")
	if err != nil {
		t.Fatalf("tmpdir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	t.Setenv("PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS", "true")

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := store.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	// 2. Bootstrap the audit user the evolver needs.
	ctx := context.Background()
	if err := db.CreateUser(ctx, selfEvolveAuditUser); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// 3. Seed a capability with a deliberately-bad prompt
	//    and a dataset that expects "pong".
	capabilityID := "cap-e2e"
	datasetID := "ds-e2e"
	capID, err := seedCapabilityWithBadPrompt(ctx, db, capabilityID, datasetID)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if capID != capabilityID {
		t.Fatalf("capID = %q, want %q", capID, capabilityID)
	}

	// 4. Wire the orchestrator with a fake LLM that always
	//    returns "pong" (the dataset's expected output).
	loader := selfevolve.NewCasPromptLoader()
	invoke := func(_ context.Context, _ selfevolve.LLMInvokeRequest) (string, error) {
		return "pong", nil
	}
	revision := selfevolve.NewLLMRevisionStrategy(invoke)
	// 5. The fakeRepository implements the evolver's
	//    Repository interface; the validator uses the
	//    same LLM and gets a 1.0 score every call.
	fake := &fakeEvolverRepo{db: db, capID: capabilityID, datasetID: datasetID, validator: invoke}
	// 6. Seed an eval run with 0/3 (score 0) so the evolver
	//    has a real failure to fix.
	if err := seedFailedEval(ctx, fake, 0, 3); err != nil {
		t.Fatalf("seed failed eval: %v", err)
	}
	validator := selfevolve.NewHarnessValidator(fake, invoke)
	sharedAuditor := &fakeAuditor{}
	promoter, perr := selfevolve.NewPromoter(fake, loader, &fakeActivator{repo: fake}, sharedAuditor)
	if perr != nil {
		t.Fatalf("NewPromoter: %v", perr)
	}
	ev := selfevolve.NewEvolver(fake, loader, revision, validator, promoter, sharedAuditor, nil)

	// 7. Set the capability's self-evolve config.
	if err := db.UpdateSelfEvolveConfig(ctx, capID, capability.SelfEvolveConfig{
		Enabled:      true,
		MinScore:     0.9,
		MaxRevisions: 3,
		CooldownSec:  0,
		TargetEnv:    "dev",
		DatasetID:    datasetID,
	}); err != nil {
		t.Fatalf("update self-evolve config: %v", err)
	}

	// 8. Run the cycle. We expect: 1 revision, validated at
	//    1.0, promoted.
	res, err := ev.RunOnce(ctx, capID)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !res.Promoted {
		t.Fatalf("expected promoted, got %+v (reject_reason=%q)", res, res.RejectReason)
	}
	if res.Revisions != 1 {
		t.Errorf("revisions = %d, want 1", res.Revisions)
	}
	if res.Score != 1.0 {
		t.Errorf("score = %v, want 1.0", res.Score)
	}

	// 9. The audit chain must have detect / revise / validate / promote.
	want := map[string]bool{
		"self_evolve.detect":   false,
		"self_evolve.revise":   false,
		"self_evolve.validate": false,
		"self_evolve.promote":  false,
	}
	for _, a := range sharedAuditor.rows {
		if _, ok := want[a.Action]; ok {
			want[a.Action] = true
		}
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("expected audit action %q, not seen", k)
		}
	}

	// 10. The state row reflects the promotion.
	state, err := db.LoadSelfEvolveState(ctx, capID, "dev")
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.LastStatus != "promoted" {
		t.Errorf("state.LastStatus = %q, want promoted", state.LastStatus)
	}
	if state.LastScore != 1.0 {
		t.Errorf("state.LastScore = %v, want 1.0", state.LastScore)
	}

	// 11. The active release is the new one, not the seeded old one.
	active, err := db.GetActiveReleaseIDInEnv(ctx, capID, "dev")
	if err != nil {
		t.Fatalf("active: %v", err)
	}
	if active == "" {
		t.Errorf("no active release")
	}
	if active == "rel-old" {
		t.Errorf("active release is still the seeded old one — promote didn't supersede it")
	}
}

// TestSelfEvolve_RejectsWhenLLMFails is the negative path:
// even with a generous threshold, if the LLM cannot
// produce the expected output the cycle rejects after
// max_revisions.
func TestSelfEvolve_RejectsWhenLLMFails(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "selfevolve-reject-")
	if err != nil {
		t.Fatalf("tmpdir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	t.Setenv("PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS", "true")

	db, err := store.NewSQLite(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.CreateUser(ctx, selfEvolveAuditUser); err != nil {
		t.Fatalf("user: %v", err)
	}
	capID, datasetID := "cap-r", "ds-r"
	if _, err := seedCapabilityWithBadPrompt(ctx, db, capID, datasetID); err != nil {
		t.Fatalf("seed: %v", err)
	}
	fake := &fakeEvolverRepo{db: db, capID: capID, datasetID: datasetID, validator: nil}
	if err := seedFailedEval(ctx, fake, 0, 3); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// LLM always returns "wrong" — validation will never pass.
	invoke := func(_ context.Context, _ selfevolve.LLMInvokeRequest) (string, error) { return "wrong", nil }
	revision := selfevolve.NewLLMRevisionStrategy(invoke)
	loader := selfevolve.NewCasPromptLoader()
	validator := selfevolve.NewHarnessValidator(fake, invoke)
	promoter, perr := selfevolve.NewPromoter(fake, loader, &fakeActivator{repo: fake}, &fakeAuditor{})
	if perr != nil {
		t.Fatalf("NewPromoter: %v", perr)
	}
	ev := selfevolve.NewEvolver(fake, loader, revision, validator, promoter, &fakeAuditor{}, nil)
	if err := db.UpdateSelfEvolveConfig(ctx, capID, capability.SelfEvolveConfig{
		Enabled: true, MinScore: 0.9, MaxRevisions: 3, CooldownSec: 0, TargetEnv: "dev", DatasetID: datasetID,
	}); err != nil {
		t.Fatalf("config: %v", err)
	}
	res, err := ev.RunOnce(ctx, capID)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if res.Promoted {
		t.Errorf("expected rejected (LLM always wrong), got %+v", res)
	}
	if res.Revisions != 3 {
		t.Errorf("revisions = %d, want 3 (max_revisions)", res.Revisions)
	}
	state, _ := db.LoadSelfEvolveState(ctx, capID, "dev")
	if state.LastStatus != "rejected" {
		t.Errorf("state.LastStatus = %q, want rejected", state.LastStatus)
	}
}

// selfEvolveAuditUser is the user the audit chain needs
// to FK-reference every self_evolve.* row. Tests create
// it at the start so audit appends don't violate the
// users FK on audit_entries.
var selfEvolveAuditUser = &models.User{
	Email: "self_evolve@e2e",
	Name:  "self_evolve",
	Role:  "admin",
}

// fakeEvolverRepo is an in-memory + SQLite hybrid. It uses
// the real SQLite for capability / version / release /
// dataset / eval persistence (so the audit chain, the
// self_evolve_state table, and the capability.SelfEvolve
// merge all behave like production), and the real CAS
// for the prompt blobs. The only thing it fakes is the
// LLM call (passed in via the validator + revision).
type fakeEvolverRepo struct {
	db        *store.SQLite
	capID     string
	datasetID string
	validator selfevolve.LLMInvokeFn
}

// All the methods below are passthroughs to db (or to the
// fakeValidator) so the evolver's Repository surface is
// fully satisfied.

func (f *fakeEvolverRepo) GetCapability(ctx context.Context, id string) (*capability.Capability, error) {
	return f.db.GetCapability(ctx, id)
}
func (f *fakeEvolverRepo) GetVersion(ctx context.Context, id string) (*capability.Version, error) {
	return f.db.GetVersion(ctx, id)
}
func (f *fakeEvolverRepo) GetVersionByNumber(ctx context.Context, capID string, n int) (*capability.Version, error) {
	return f.db.GetVersionByNumber(ctx, capID, n)
}
func (f *fakeEvolverRepo) CreateVersion(ctx context.Context, v *capability.Version) error {
	return f.db.CreateVersion(ctx, v)
}
func (f *fakeEvolverRepo) UpdateSelfEvolveConfig(ctx context.Context, id string, cfg capability.SelfEvolveConfig) error {
	return f.db.UpdateSelfEvolveConfig(ctx, id, cfg)
}
func (f *fakeEvolverRepo) LoadSelfEvolveState(ctx context.Context, capID, env string) (*store.SelfEvolveState, error) {
	return f.db.LoadSelfEvolveState(ctx, capID, env)
}
func (f *fakeEvolverRepo) SaveSelfEvolveState(ctx context.Context, st *store.SelfEvolveState) error {
	return f.db.SaveSelfEvolveState(ctx, st)
}
func (f *fakeEvolverRepo) GetActiveReleaseID(ctx context.Context, capID string) (string, error) {
	return f.db.GetActiveReleaseID(ctx, capID)
}
func (f *fakeEvolverRepo) ActiveReleaseID(ctx context.Context, capID, env string) (string, error) {
	return f.db.GetActiveReleaseIDInEnv(ctx, capID, env)
}
func (f *fakeEvolverRepo) GetRelease(ctx context.Context, id string) (*selfevolve.ReleaseRecord, error) {
	rel, err := f.db.GetRelease(ctx, id)
	if err != nil || rel == nil {
		return nil, err
	}
	return &selfevolve.ReleaseRecord{
		ID: rel.ID, CapabilityID: rel.CapabilityID, CapabilityVersion: rel.CapabilityVersion,
		Manifest: rel.Manifest, Environment: string(rel.Environment), Status: string(rel.Status),
		CreatedBy: rel.CreatedBy, CreatedAt: rel.CreatedAt,
	}, nil
}
func (f *fakeEvolverRepo) LastEvalRun(ctx context.Context, releaseID string) (*harness.EvalRun, error) {
	return f.db.LastEvalRunForRelease(ctx, releaseID)
}
func (f *fakeEvolverRepo) UpdateReleaseStatus(ctx context.Context, releaseID, status string) error {
	rel, err := f.db.GetRelease(ctx, releaseID)
	if err != nil || rel == nil {
		return err
	}
	rel.Status = release.Status(status)
	return f.db.UpdateRelease(ctx, rel)
}
func (f *fakeEvolverRepo) CreateRelease(ctx context.Context, rec selfevolve.ReleaseRecord) error {
	r := &release.Release{
		ID: rec.ID, CapabilityID: rec.CapabilityID, CapabilityVersion: rec.CapabilityVersion,
		Manifest: rec.Manifest, Environment: release.Environment(rec.Environment),
		Status: release.Status(rec.Status), CreatedBy: rec.CreatedBy, CreatedAt: rec.CreatedAt,
	}
	return f.db.CreateRelease(ctx, r)
}
func (f *fakeEvolverRepo) CreateDataset(ctx context.Context, d *harness.Dataset) error {
	return f.db.CreateDataset(ctx, d)
}
func (f *fakeEvolverRepo) GetDataset(ctx context.Context, id string) (*harness.Dataset, error) {
	return f.db.GetDataset(ctx, id)
}
func (f *fakeEvolverRepo) ListDatasetsForCapability(ctx context.Context, capID string) ([]*harness.Dataset, error) {
	return f.db.ListDatasetsForCapability(ctx, capID)
}
func (f *fakeEvolverRepo) DeleteDataset(ctx context.Context, id string) error {
	return f.db.DeleteDataset(ctx, id)
}
func (f *fakeEvolverRepo) UpsertDatasetCases(ctx context.Context, datasetID string, cases []harness.DatasetCase) error {
	return f.db.UpsertDatasetCases(ctx, datasetID, cases)
}
func (f *fakeEvolverRepo) CreatePrecondition(ctx context.Context, p *harness.Precondition) error {
	return f.db.CreatePrecondition(ctx, p)
}
func (f *fakeEvolverRepo) GetPrecondition(ctx context.Context, id string) (*harness.Precondition, error) {
	return f.db.GetPrecondition(ctx, id)
}
func (f *fakeEvolverRepo) ListPreconditionsForCapability(ctx context.Context, capID string) ([]*harness.Precondition, error) {
	return f.db.ListPreconditionsForCapability(ctx, capID)
}
func (f *fakeEvolverRepo) UpdatePrecondition(ctx context.Context, p *harness.Precondition) error {
	return f.db.UpdatePrecondition(ctx, p)
}
func (f *fakeEvolverRepo) DeletePrecondition(ctx context.Context, id string) error {
	return f.db.DeletePrecondition(ctx, id)
}
func (f *fakeEvolverRepo) CreateEvalRun(ctx context.Context, run *harness.EvalRun) error {
	return f.db.CreateEvalRun(ctx, run)
}
func (f *fakeEvolverRepo) UpdateEvalRun(ctx context.Context, run *harness.EvalRun) error {
	return f.db.UpdateEvalRun(ctx, run)
}
func (f *fakeEvolverRepo) CreateEvalResults(ctx context.Context, rs []harness.EvalResult) error {
	return f.db.CreateEvalResults(ctx, rs)
}
func (f *fakeEvolverRepo) CreateEvalResult(ctx context.Context, r *harness.EvalResult) error {
	return f.db.CreateEvalResult(ctx, r)
}
func (f *fakeEvolverRepo) ListDatasetCases(ctx context.Context, datasetID string) ([]harness.DatasetCase, error) {
	return f.db.ListDatasetCases(ctx, datasetID)
}
func (f *fakeEvolverRepo) ListEvalResultsForRun(ctx context.Context, runID string) ([]harness.EvalResult, error) {
	return f.db.ListEvalResultsForRun(ctx, runID)
}
func (f *fakeEvolverRepo) ListEvalRunsForRelease(ctx context.Context, releaseID string) ([]*harness.EvalRun, error) {
	return f.db.ListEvalRunsForRelease(ctx, releaseID)
}
func (f *fakeEvolverRepo) GetEvalRun(ctx context.Context, id string) (*harness.EvalRun, error) {
	return f.db.GetEvalRun(ctx, id)
}

// fakeActivator's SelfActivate is called by the promoter.
// It supersedes the prior active release in the same env
// and marks the new one active — the same logic as
// release.Service.SelfActivate.
type fakeActivator struct{ repo *fakeEvolverRepo }

func (a *fakeActivator) SelfActivate(ctx context.Context, releaseID string) error {
	rel, err := a.repo.db.GetRelease(ctx, releaseID)
	if err != nil {
		return err
	}
	// Supersede any prior active in the same env.
	prior, err := a.repo.db.GetActiveReleaseIDInEnv(ctx, rel.CapabilityID, string(rel.Environment))
	if err == nil && prior != "" && prior != releaseID {
		p, perr := a.repo.db.GetRelease(ctx, prior)
		if perr == nil && p != nil {
			p.Status = release.StatusSuperseded
			_ = a.repo.db.UpdateRelease(ctx, p)
		}
	}
	rel.Status = release.StatusActive
	return a.repo.db.UpdateRelease(ctx, rel)
}

// fakeAuditor is a test double for selfevolve.Auditor
// that records every audit call.
type fakeAuditor struct{ rows []fakeAuditRow }

type fakeAuditRow struct {
	Action string
	Target string
}

func (a *fakeAuditor) Audit(_ context.Context, action, target string, _ map[string]any) {
	a.rows = append(a.rows, fakeAuditRow{action, target})
}

// seedCapabilityWithBadPrompt creates a capability with v-1
// whose prompt is "audit code" (so the dataset's "pong"
// expected values won't match), an active release in
// env=dev, and a 3-case dataset that expects "pong".
func seedCapabilityWithBadPrompt(ctx context.Context, db *store.SQLite, capID, datasetID string) (string, error) {
	ws := "ws-" + capID
	if err := db.CreateWorkspace(ctx, &capability.Workspace{ID: ws, Name: "e2e-ws"}); err != nil {
		return "", fmt.Errorf("create workspace: %w", err)
	}
	proj := "p-" + capID
	if err := db.CreateProject(ctx, &capability.Project{ID: proj, WorkspaceID: ws, Name: "e2e-p"}); err != nil {
		return "", fmt.Errorf("create project: %w", err)
	}
	if err := db.CreateCapability(ctx, &capability.Capability{ID: capID, ProjectID: proj, Name: "e2e-cap"}); err != nil {
		return "", fmt.Errorf("create capability: %w", err)
	}
	// Bad prompt blob.
	badText := "audit code"
	badHash, _ := casWritePrompt(badText, "")
	mpHash := "mp-" + capID
	rtHash := "rt-" + capID
	manifest := capability.Manifest{
		Prompt:        capability.ArtifactRef{Kind: capability.ArtifactPrompt, Hash: badHash},
		ModelPolicy:   capability.ArtifactRef{Kind: capability.ArtifactModelPolicy, Hash: mpHash},
		RuntimePolicy: capability.ArtifactRef{Kind: capability.ArtifactRuntimePolicy, Hash: rtHash},
		Context:       capability.ArtifactRef{Kind: capability.ArtifactContext, Hash: rtHash},
		Memory:        capability.ArtifactRef{Kind: capability.ArtifactMemory, Hash: rtHash},
	}
	mh, _ := capability.ComputeManifestHash(manifest)
	vid := "v-1-" + capID
	if err := db.CreateVersion(ctx, &capability.Version{
		ID: vid, CapabilityID: capID, Version: 1, Manifest: manifest, ManifestHash: mh,
		CreatedAt: time.Now().UTC(), CreatedBy: "seed",
	}); err != nil {
		return "", fmt.Errorf("create version: %w", err)
	}
	// CAS write the bad prompt.
	if err := writeCASPrompt(badText, badHash); err != nil {
		return "", err
	}
	// Active release in dev.
	relID := "rel-old"
	if err := db.CreateRelease(ctx, &release.Release{
		ID: relID, CapabilityID: capID, CapabilityVersion: 1, Manifest: manifest,
		Environment: release.Environment("dev"), Status: release.StatusActive,
		CreatedBy: "seed", CreatedAt: time.Now().UTC(),
	}); err != nil {
		return "", fmt.Errorf("create release: %w", err)
	}
	// Dataset with 3 cases expecting "pong".
	ds := &harness.Dataset{ID: datasetID, CapabilityID: capID, Name: datasetID}
	if err := db.CreateDataset(ctx, ds); err != nil {
		return "", fmt.Errorf("create dataset: %w", err)
	}
	if err := db.UpsertDatasetCases(ctx, datasetID, []harness.DatasetCase{
		{ID: "k0", DatasetID: datasetID, Seq: 0, Inputs: []byte(`{"q":"ping"}`), Expected: []byte(`"pong"`)},
		{ID: "k1", DatasetID: datasetID, Seq: 1, Inputs: []byte(`{"q":"hi"}`), Expected: []byte(`"pong"`)},
		{ID: "k2", DatasetID: datasetID, Seq: 2, Inputs: []byte(`{"q":"yo"}`), Expected: []byte(`"pong"`)},
	}); err != nil {
		return "", fmt.Errorf("upsert dataset cases: %w", err)
	}
	// Approval so SelfActivate doesn't fail.
	if err := db.CreateApproval(ctx, &approval.Approval{
		ReleaseID: relID, UpdatedAt: time.Now().UTC(),
	}); err != nil {
		return "", fmt.Errorf("create approval: %w", err)
	}
	return capID, nil
}

// seedFailedEval inserts a finished eval run with passed/0
// and failed/total so the score is 0.
func seedFailedEval(ctx context.Context, f *fakeEvolverRepo, passed, failed int) error {
	runID := fmt.Sprintf("erun-%d", time.Now().UnixNano())
	run := &harness.EvalRun{
		ID: runID, ReleaseID: "rel-old", DatasetID: f.datasetID, Scorer: "contains",
		StartedAt: time.Now().UTC(),
	}
	if err := f.db.CreateEvalRun(ctx, run); err != nil {
		return err
	}
	now := time.Now().UTC()
	var results []harness.EvalResult
	for i := 0; i < passed; i++ {
		results = append(results, harness.EvalResult{
			ID: fmt.Sprintf("%s-r%d", runID, i), RunID: runID, CaseID: fmt.Sprintf("k%d", i), Seq: i, Passed: true,
		})
	}
	for i := 0; i < failed; i++ {
		seq := passed + i
		results = append(results, harness.EvalResult{
			ID: fmt.Sprintf("%s-r%d", runID, seq), RunID: runID, CaseID: fmt.Sprintf("k%d", seq), Seq: seq,
			Actual: []byte(`"wrong"`),
		})
	}
	run.Total = passed + failed
	run.Passed = passed
	run.Failed = failed
	if run.Total > 0 {
		run.Score = float64(run.Passed) / float64(run.Total)
	}
	if run.Failed == 0 {
		run.Status = harness.RunPassed
	} else {
		run.Status = harness.RunFailed
	}
	run.FinishedAt = &now
	if err := f.db.UpdateEvalRun(ctx, run); err != nil {
		return err
	}
	return f.db.CreateEvalResults(ctx, results)
}

// writeCASPrompt writes a prompt blob to the local CAS at
// .promptsheon/objects/. Tests must chdir to a temp dir
// before calling this.
func writeCASPrompt(text, hash string) error {
	_, err := casWritePrompt(text, hash)
	return err
}
