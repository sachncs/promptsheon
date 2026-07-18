package harness_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/eval"
	"github.com/sachncs/promptsheon/internal/harness"
	"github.com/sachncs/promptsheon/internal/testdata"
)

// stubInvoker is a ReleaseInvoker that returns a canned output for
// every case. The optional mutate hook lets a test inject per-case
// output variations.
type stubInvoker struct {
	fixed json.RawMessage
	err   error
}

func (s *stubInvoker) Invoke(_ context.Context, _ string, _ map[string]any) (json.RawMessage, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.fixed, nil
}

func TestEvalRunnerHappyPath(t *testing.T) {
	now := time.Now().UTC()
	r := harness.NewEvalRunner(nil, nil)
	r.Clock = func() time.Time { return now }
	r.Repo = newRepo(t)

	dataset := mustCreateDataset(t, r.Repo, "c1", "greeting", []harness.DatasetCase{
		{ID: "c1-case1", Seq: 0, Inputs: json.RawMessage(`"hi"`), Expected: json.RawMessage(`"hi"`)},
		{ID: "c1-case2", Seq: 1, Inputs: json.RawMessage(`"hi"`), Expected: json.RawMessage(`"hi"`)},
	})

	r.Inv = &stubInvoker{fixed: json.RawMessage(`"hi"`)}
	run, err := r.Run(context.Background(), harness.EvalRunOptions{
		ReleaseID:  "r1",
		DatasetID:  dataset,
		ScorerName: eval.ScorerExactMatch,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if run.Status != harness.RunPassed {
		t.Fatalf("status = %q want passed", run.Status)
	}
	if run.Total != 2 || run.Passed != 2 || run.Failed != 0 {
		t.Fatalf("counts: total=%d passed=%d failed=%d", run.Total, run.Passed, run.Failed)
	}
	if run.Score != 1.0 {
		t.Fatalf("score = %v want 1.0", run.Score)
	}
	if run.StartedAt.IsZero() || run.FinishedAt == nil {
		t.Fatalf("started_at=%v finished_at=%v", run.StartedAt, run.FinishedAt)
	}

	results, err := r.Repo.ListEvalResultsForRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("list results: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results want 2", len(results))
	}
	for _, x := range results {
		if !x.Passed {
			t.Errorf("result %s not passed: %s", x.ID, x.Error)
		}
	}
}

func TestEvalRunnerMixedResults(t *testing.T) {
	now := time.Now().UTC()
	r := harness.NewEvalRunner(nil, nil)
	r.Clock = func() time.Time { return now }
	r.Repo = newRepo(t)

	dataset := mustCreateDataset(t, r.Repo, "c1", "greet", []harness.DatasetCase{
		{ID: "c1-pass", Seq: 0, Inputs: json.RawMessage(`"x"`), Expected: json.RawMessage(`"hi"`)},
		{ID: "c1-fail", Seq: 1, Inputs: json.RawMessage(`"y"`), Expected: json.RawMessage(`"bye"`)},
	})
	r.Inv = &stubInvoker{fixed: json.RawMessage(`"hi"`)} // only matches first case

	run, err := r.Run(context.Background(), harness.EvalRunOptions{
		ReleaseID:  "r1",
		DatasetID:  dataset,
		ScorerName: eval.ScorerExactMatch,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if run.Status != harness.RunFailed {
		t.Fatalf("status = %q want failed", run.Status)
	}
	if run.Passed != 1 || run.Failed != 1 || run.Total != 2 {
		t.Fatalf("counts: total=%d passed=%d failed=%d", run.Total, run.Passed, run.Failed)
	}
	if run.Score != 0.5 {
		t.Fatalf("score = %v want 0.5", run.Score)
	}
}

func TestEvalRunnerInvokerError(t *testing.T) {
	now := time.Now().UTC()
	r := harness.NewEvalRunner(nil, nil)
	r.Clock = func() time.Time { return now }
	r.Repo = newRepo(t)

	dataset := mustCreateDataset(t, r.Repo, "c1", "x", []harness.DatasetCase{
		{ID: "c1", Seq: 0, Inputs: json.RawMessage(`{}`), Expected: json.RawMessage(`"x"`)},
	})
	r.Inv = &stubInvoker{err: errors.New("invoke boom")}

	run, err := r.Run(context.Background(), harness.EvalRunOptions{
		ReleaseID:  "r1",
		DatasetID:  dataset,
		ScorerName: eval.ScorerExactMatch,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if run.Status != harness.RunFailed {
		t.Fatalf("status = %q want failed", run.Status)
	}
	results, _ := r.Repo.ListEvalResultsForRun(context.Background(), run.ID)
	if len(results) != 1 || results[0].Error == "" || results[0].Passed {
		t.Fatalf("expected one failed result with error, got %+v", results)
	}
}

func TestEvalRunnerUnknownScorer(t *testing.T) {
	r := harness.NewEvalRunner(nil, &stubInvoker{})
	r.Repo = newRepo(t)
	dataset := mustCreateDataset(t, r.Repo, "c1", "x", []harness.DatasetCase{
		{ID: "c1", Seq: 0, Inputs: json.RawMessage(`{}`), Expected: json.RawMessage(`"x"`)},
	})
	_, err := r.Run(context.Background(), harness.EvalRunOptions{
		ReleaseID:  "r1",
		DatasetID:  dataset,
		ScorerName: eval.Scorer("does_not_exist"),
	})
	if err == nil {
		t.Fatal("expected error for unknown scorer")
	}
}

// ----- helpers -----

func mustCreateDataset(t *testing.T, repo harness.Repository, capabilityID, name string, cases []harness.DatasetCase) string {
	t.Helper()
	d := &harness.Dataset{
		ID:           "ds-" + name,
		CapabilityID: capabilityID,
		Name:         name,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := repo.CreateDataset(context.Background(), d); err != nil {
		t.Fatalf("CreateDataset: %v", err)
	}
	if err := repo.UpsertDatasetCases(context.Background(), d.ID, cases); err != nil {
		t.Fatalf("UpsertDatasetCases: %v", err)
	}
	return d.ID
}

// silence unused import warning for testdata (some test helpers live in other files)
var _ = testdata.NewManifest
