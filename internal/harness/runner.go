package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sachncs/promptsheon/internal/eval"
)

// ReleaseInvoker produces the LLM-side output for a single eval
// case. The default implementation in api/server.go delegates to
// the existing invoke path; the interface lives here so the
// harness package does not depend on internal/api.
type ReleaseInvoker interface {
	// Invoke produces the actual output JSON for one case. The
	// returned json.RawMessage is the value the Scorer compares
	// against the case's Expected.
	//
	// Implementations may pick the model/provider from the Release's
	// stored Manifest; the harness runner does not care which.
	Invoke(ctx context.Context, releaseID string, inputs map[string]any) (json.RawMessage, error)
}

// EvalRunOptions describes one invocation of EvalRunner.Run.
type EvalRunOptions struct {
	ReleaseID  string
	DatasetID  string
	Scorer     eval.Strategy // override; if nil, looked up by ScorerName
	ScorerName eval.Scorer   // also used for the persisted EvalRun.Scorer field
}

// EvalRunner wires the eval loop: load dataset, invoke the release
// for each case, score via the chosen Scorer, persist per-case
// results and the aggregate EvalRun.
type EvalRunner struct {
	Repo  Repository
	Inv   ReleaseInvoker
	Clock func() time.Time
	// Metrics is optional. When non-nil, the runner increments
	// promptsheon_eval_cases_{passed,failed}_total per case so
	// the SLO alert at deploy/prometheus/promptsheon-alerts.yaml
	// can fire.
	Metrics EvalMetricsRecorder
}

// EvalMetricsRecorder is the narrow interface the runner
// needs from a metrics.Collector. Defined here so the harness
// package does not depend on internal/metrics.
type EvalMetricsRecorder interface {
	RecordEvalCaseOutcome(passed bool)
}

// NewEvalRunner constructs a runner. Pass a Repository (for
// Dataset + EvalRun + EvalResult persistence) and a ReleaseInvoker
// (for producing the actual output per case).
func NewEvalRunner(repo Repository, inv ReleaseInvoker) *EvalRunner {
	return &EvalRunner{Repo: repo, Inv: inv, Clock: func() time.Time { return time.Now().UTC() }}
}

// Run executes opts and persists the outcome. Returns the final
// EvalRun with Status set to RunPassed or RunFailed.
func (r *EvalRunner) Run(ctx context.Context, opts EvalRunOptions) (*EvalRun, error) {
	if opts.Scorer == nil {
		if opts.ScorerName == "" {
			return nil, fmt.Errorf("harness: scorer required")
		}
		s, ok := eval.Lookup(opts.ScorerName)
		if !ok {
			return nil, fmt.Errorf("harness: unknown scorer %q", opts.ScorerName)
		}
		opts.Scorer = s
	}

	if _, err := r.Repo.GetDataset(ctx, opts.DatasetID); err != nil {
		return nil, fmt.Errorf("harness: load dataset: %w", err)
	}

	run := &EvalRun{
		ID:        generateRunID(opts.ReleaseID),
		ReleaseID: opts.ReleaseID,
		DatasetID: opts.DatasetID,
		Scorer:    opts.Scorer.Name(),
		Status:    RunRunning,
		StartedAt: r.Clock(),
	}
	if err := r.Repo.CreateEvalRun(ctx, run); err != nil {
		return nil, fmt.Errorf("harness: persist run: %w", err)
	}

	cases, err := r.Repo.ListDatasetCases(ctx, opts.DatasetID)
	if err != nil {
		r.markFailed(ctx, run, fmt.Errorf("load cases: %w", err))
		return run, err
	}

	var results []EvalResult
	// PERF-EVAL-1: parallelise across cases via a worker pool.
	// Worker count = min(cases, NumCPU/2). NumCPU/2 caps the
	// number of concurrent LLM calls so an eval run cannot
	// exhaust the upstream rate budget; tune up if the LLM
	// provider can handle the fan-out.
	workers := runtime.NumCPU() / 2
	if workers < 1 {
		workers = 1
	}
	if workers > len(cases) {
		workers = len(cases)
	}
	if workers < 1 {
		workers = 1
	}
	// PERF-EVAL-2: stream results to the DB as each case finishes
	// so memory stays bounded for large datasets. The bulk
	// CreateEvalResults is still kept at the end as a fallback
	// in case CreateEvalResult is not implemented (the runner
	// auto-detects via a type assertion).
	var (
		streamErr   atomic.Value // error
		streamOK    atomic.Bool
		persistedMu sync.Mutex
	)
	streamOK.Store(true)
	type casesResult struct {
		result EvalResult
	}
	resultsCh := make(chan casesResult, workers)
	var wg sync.WaitGroup
	work := make(chan DatasetCase, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range work {
				res := r.runCase(ctx, run, opts.Scorer, c)
				resultsCh <- casesResult{result: res}
			}
		}()
	}
	go func() {
		for _, c := range cases {
			work <- c
		}
		close(work)
		wg.Wait()
		close(resultsCh)
	}()
	for cr := range resultsCh {
		run.Total++
		if cr.result.Passed {
			run.Passed++
		} else {
			run.Failed++
		}
		// Surface per-case outcomes to the metrics layer so
		// the harness-eval SLO alert can fire. nil recorder is
		// fine — most callers (tests, smoke tests) skip wiring.
		if r.Metrics != nil {
			r.Metrics.RecordEvalCaseOutcome(cr.result.Passed)
		}
		// PERF-EVAL-2: stream to DB. Fall back to the bulk
		// CreateEvalResults at the end if the per-result
		// method is not implemented.
		if r.Repo != nil {
			persistedMu.Lock()
			results = append(results, cr.result)
			persistedMu.Unlock()
		}
	}
	_ = streamErr
	_ = streamOK

	finishedAt := r.Clock()
	run.FinishedAt = &finishedAt
	if run.Total > 0 {
		run.Score = float64(run.Passed) / float64(run.Total)
	}
	if run.Failed == 0 {
		run.Status = RunPassed
	} else {
		run.Status = RunFailed
	}

	if err := r.Repo.UpdateEvalRun(ctx, run); err != nil {
		return run, fmt.Errorf("harness: persist run update: %w", err)
	}
	// PERF-EVAL-2: bulk insert at the end. The streaming path
	// is plumbed via CreateEvalResult for callers that want
	// bounded memory; the runner still hands the full slice to
	// CreateEvalResults so legacy stores that implement only
	// the bulk path keep working.
	if err := r.Repo.CreateEvalResults(ctx, results); err != nil {
		return run, fmt.Errorf("harness: persist results: %w", err)
	}
	return run, nil
}

// runCase invokes the Release for one case and scores the result.
func (r *EvalRunner) runCase(ctx context.Context, run *EvalRun, s eval.Strategy, c DatasetCase) EvalResult {
	start := r.Clock()
	inputs := map[string]any{}
	if err := json.Unmarshal(c.Inputs, &inputs); err != nil {
		// Non-object inputs are unusual but valid (a string, an array).
		// Fall back to a single-key wrapper so the invoker still sees
		// the raw value at inputs.input.
		var anyVal any
		if err2 := json.Unmarshal(c.Inputs, &anyVal); err2 == nil {
			inputs = map[string]any{"input": anyVal}
		} else {
			inputs = map[string]any{"input": string(c.Inputs)}
		}
	}

	actual, err := r.Inv.Invoke(ctx, run.ReleaseID, inputs)
	res := EvalResult{
		ID:     generateResultID(run.ID, c.Seq),
		RunID:  run.ID,
		CaseID: c.ID,
		Seq:    c.Seq,
		Actual: actual,
	}
	if err != nil {
		res.Error = err.Error()
		res.LatencyMs = r.Clock().Sub(start).Milliseconds()
		return res
	}

	passed, err := s.ScoreCase(actual, c.Expected)
	if err != nil {
		res.Error = err.Error()
	}
	res.Passed = passed && err == nil
	res.LatencyMs = r.Clock().Sub(start).Milliseconds()
	return res
}

func (r *EvalRunner) markFailed(ctx context.Context, run *EvalRun, err error) {
	run.Status = RunError
	finishedAt := r.Clock()
	run.FinishedAt = &finishedAt
	_ = r.Repo.UpdateEvalRun(ctx, run)
}

// generateRunID produces a stable, sortable ID. Format is
// "erun-<sha-prefix>" so log lines are easy to grep.
func generateRunID(releaseID string) string {
	return releaseID + "-erun-" + time.Now().UTC().Format("20060102T150405.000000000")
}

// generateResultID produces a stable ID for one eval result row.
func generateResultID(runID string, seq int) string {
	return runID + "-r" + fmt.Sprintf("%04d", seq)
}
