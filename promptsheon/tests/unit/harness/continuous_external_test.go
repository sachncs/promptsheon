package harness_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/promptsheon/harness"
	"github.com/sachncs/promptsheon/promptsheon/release"
	"github.com/sachncs/promptsheon/promptsheon/testutil/harnessrepo"
)

// fakeInvoker returns a fixed output for every Invoke call.
// The output is the case's input wrapped in `{"echo": "..."}`
// so an exact_match scorer with expected `{"echo":"hi"}`
// produces a passing case.
type fakeInvoker struct {
	output map[string]any
}

func (f *fakeInvoker) Invoke(_ context.Context, _ string, inputs map[string]any) (json.RawMessage, error) {
	if f.output != nil {
		b, _ := json.Marshal(f.output)
		return b, nil
	}
	b, _ := json.Marshal(inputs)
	return b, nil
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestContinuousEvalDisabledWithZeroInterval(t *testing.T) {
	repo := harnessrepo.New()
	c := harness.NewContinuousEval(harness.ContinuousEvalConfig{
		CapabilityID: "c1",
		Interval:     0,
	}, repo, nil, newTestLogger())
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	c.Stop() // must not block
}

func TestContinuousEvalRunOnceNoActiveRelease(t *testing.T) {
	repo := harnessrepo.New()
	c := harness.NewContinuousEval(harness.ContinuousEvalConfig{
		CapabilityID: "c1",
		DatasetID:    "d1",
		Interval:     time.Hour, // long enough that the loop never fires during the test
	}, repo, harness.NewEvalRunner(repo, &fakeInvoker{}), newTestLogger())
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()
	// runOnce is a method we can drive directly: with no
	// active release in the repo it should be a no-op.
	c.RunOnce(context.Background())
	// No assertion: nothing should have happened.
}

func TestContinuousEvalRunOnceWithActiveReleaseAndDataset(t *testing.T) {
	repo := harnessrepo.New()
	// Seed an active release.
	now := time.Now().UTC()
	repo.Releases["rel-1"] = &release.Release{
		ID: "rel-1", CapabilityID: "c1",
		Status:      release.StatusActive,
		ActivatedAt: &now,
	}
	// Seed a dataset with one case.
	repo.Datasets["d1"] = &harness.Dataset{ID: "d1", CapabilityID: "c1"}
	repo.Cases["d1"] = []harness.DatasetCase{
		{ID: "case-1", Seq: 0,
			Inputs:   json.RawMessage(`{"text":"hi"}`),
			Expected: json.RawMessage(`{"echo":"hi"}`),
		},
	}
	inv := &fakeInvoker{output: map[string]any{"echo": "hi"}}
	runner := harness.NewEvalRunner(repo, inv)
	c := harness.NewContinuousEval(harness.ContinuousEvalConfig{
		CapabilityID: "c1",
		DatasetID:    "d1",
		Interval:     time.Hour,
		ScorerName:   "exact_match",
	}, repo, runner, newTestLogger())
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()
	c.RunOnce(context.Background())
	// The runner should have produced one EvalRun with one
	// passing case.
	runs, err := repo.ListEvalRunsForRelease(context.Background(), "rel-1")
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 eval run, got %d", len(runs))
	}
	if runs[0].Status != harness.RunPassed {
		t.Errorf("expected run passed, got %s", runs[0].Status)
	}
}

func TestContinuousEvalStartTwiceOrAfterStop(t *testing.T) {
	repo := harnessrepo.New()
	c := harness.NewContinuousEval(harness.ContinuousEvalConfig{
		CapabilityID: "c1",
		Interval:     0,
	}, repo, nil, newTestLogger())
	c.Stop()
	if err := c.Start(context.Background()); err == nil {
		t.Error("Start after Stop must error")
	}
}

func TestContinuousEvalDefaultScorer(t *testing.T) {
	// When ScorerName is empty, the loop defaults to
	// exact_match. Pin the contract so a future refactor
	// doesn't accidentally change the default scorer.
	if got := harness.DefaultScorer(""); got != "exact_match" {
		t.Errorf("default scorer: got %q want exact_match", got)
	}
}
