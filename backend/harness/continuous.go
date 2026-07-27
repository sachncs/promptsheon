// ContinuousEval: a scheduled eval loop that runs an
// EvalRunner against the active Release of a Capability on a
// fixed cadence. The schedule is configured per-Capability;
// the runner runs the active Release's Manifest against the
// Capability's primary Dataset and persists results.
//
// PROD-CONT-1: the loop must be:
//   - cancelable (Stop on daemon shutdown)
//   - cheap when there's no work (a 1-minute cadence costs O(1))
//   - bounded (never spawn overlapping runs)
//   - observable (logs every run with the EvalRun ID)
//
// ContinuousEval does NOT persist schedules — the capability
// row carries a `ContinuousEvalIntervalSec` column; if it's 0
// the loop is a no-op for that capability. The migration in
// 019_continuous_eval adds the column.
package harness

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/sachncs/promptsheon/internal/eval"
)

// ContinuousEvalConfig describes one scheduled eval.
type ContinuousEvalConfig struct {
	CapabilityID string
	DatasetID    string
	Interval     time.Duration // 0 disables
	ScorerName   string
	// Jitter adds up to Jitter random delay before each run
	// so a fleet of replicas does not stampede the upstream
	// provider. Defaults to 0 (no jitter).
	Jitter time.Duration
}

// ContinuousEval runs EvalRunner on a fixed cadence for one
// capability. The lifecycle is: New → Start → Stop. Stop is
// idempotent and safe to call from any goroutine.
type ContinuousEval struct {
	cfg     ContinuousEvalConfig
	repo    Repository
	runner  *EvalRunner
	logger  *slog.Logger
	stop    chan struct{}
	done    chan struct{}
	stopped bool
	started bool
	mu      sync.Mutex
}

// NewContinuousEval constructs a ContinuousEval. Pass nil
// runner to disable execution (the loop still ticks but does
// nothing). The caller owns the runner.
func NewContinuousEval(cfg ContinuousEvalConfig, repo Repository, runner *EvalRunner, logger *slog.Logger) *ContinuousEval {
	if cfg.Interval < 0 {
		cfg.Interval = 0
	}
	return &ContinuousEval{
		cfg:    cfg,
		repo:   repo,
		runner: runner,
		logger: logger,
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
}

// Start kicks off the ticker. Returns immediately; the loop
// runs in a background goroutine. Calling Start twice returns
// an error.
func (c *ContinuousEval) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		return errors.New("continuous_eval: already started")
	}
	if c.stopped {
		return errors.New("continuous_eval: already stopped")
	}
	c.started = true
	if c.cfg.Interval == 0 {
		// Disabled; close done synchronously so Stop does not
		// block waiting on a goroutine that may not have been
		// scheduled yet.
		close(c.done)
		return nil
	}
	go c.loop(ctx)
	return nil
}

// Stop signals the loop to exit and waits for it. Safe to call
// multiple times.
func (c *ContinuousEval) Stop() {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		<-c.done
		return
	}
	c.stopped = true
	if !c.started {
		// Stop without Start; close done so a later Stop is
		// safe.
		close(c.done)
		c.mu.Unlock()
		return
	}
	close(c.stop)
	c.mu.Unlock()
	<-c.done
}

// loop is the tick body. It runs the first eval after the
// initial interval (NOT immediately on Start, so a daemon
// restart does not stampede the provider); subsequent runs
// fire on the configured cadence.
func (c *ContinuousEval) loop(ctx context.Context) {
	defer close(c.done)
	t := time.NewTicker(c.cfg.Interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stop:
			return
		case <-t.C:
			c.RunOnce(ctx)
		}
	}
}

// RunOnce is one scheduled eval. Public so external callers
// (tests, hot-reload paths) can drive a single iteration
// without spinning up the ticker. Errors are logged but do
// not stop the loop: a transient failure should not silently
// disable ContinuousEval.
func (c *ContinuousEval) RunOnce(ctx context.Context) {
	if c.runner == nil || c.repo == nil {
		return
	}
	// Look up the active release for the capability.
	releaseID, err := c.repo.GetActiveReleaseID(ctx, c.cfg.CapabilityID)
	if err != nil || releaseID == "" {
		// No active release — nothing to eval. This is the
		// common case between Activate cycles; not an error.
		c.logger.Debug("continuous_eval: no active release",
			"capability_id", c.cfg.CapabilityID)
		return
	}
	opts := EvalRunOptions{
		ReleaseID:  releaseID,
		DatasetID:  c.cfg.DatasetID,
		ScorerName: eval.Scorer(DefaultScorer(c.cfg.ScorerName)),
	}
	run, err := c.runner.Run(ctx, opts)
	if err != nil {
		c.logger.Warn("continuous_eval: run failed",
			"capability_id", c.cfg.CapabilityID,
			"release_id", releaseID,
			"err", err)
		return
	}
	c.logger.Info("continuous_eval: run completed",
		"capability_id", c.cfg.CapabilityID,
		"release_id", releaseID,
		"eval_run_id", run.ID,
		"score", run.Score,
		"passed", run.Passed,
		"failed", run.Failed,
		"total", run.Total)
}

// DefaultScorer returns the scorer name. The empty sentinel
// falls through to EvalRunner.Run's scorer-name lookup, which
// defaults to exact_match when unset.
func DefaultScorer(name string) string {
	if name == "" {
		return "exact_match"
	}
	return name
}
