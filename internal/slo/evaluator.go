package slo

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// Evaluator runs SLO goals against live signals at a fixed
// interval and emits a BreachEvent for every failed evaluation.
//
// Production wiring constructs one Evaluator per daemon, calls
// Start in a goroutine, and stops via the supplied context. The
// Evaluator reads SLOs through its Repository and the actual
// signal values through a SourceFunc the caller supplies.
//
// The wire format for SourceFunc is intentionally narrow (single
// float64 keyed by SLO id) so the implementation does not need to
// understand every Signal. Production sources know which SLO they
// drive and project their metric into a comparable float.
type Evaluator struct {
	repo     Repository
	source   SourceFunc
	logger   *slog.Logger
	interval time.Duration
	now      func() time.Time

	mu      sync.Mutex
	lastRun time.Time
}

// SourceFunc returns the latest observed value for the supplied
// SLO id. The function is expected to be cheap; the Evaluator
// calls it once per SLO per tick.
type SourceFunc func(ctx context.Context, s *SLO) (float64, error)

// BreachEvent is what the Evaluator emits when an SLO fails.
// Downstream consumers (alerting manager, recommendation loop)
// receive it through the callback registered via OnBreach.
type BreachEvent struct {
	SLO        *SLO
	Actual     float64
	BurnRate   float64
	ObservedAt time.Time
}

// BreachFunc is the callback invoked per failed evaluation. It
// runs on the Evaluator goroutine; implementations MUST be
// non-blocking and MUST NOT call back into the Evaluator.
type BreachFunc func(ctx context.Context, e BreachEvent) error

// NewEvaluator constructs an Evaluator. interval is the tick
// cadence; the default is one minute when zero is supplied.
func NewEvaluator(repo Repository, source SourceFunc, logger *slog.Logger, interval time.Duration) *Evaluator {
	if interval <= 0 {
		interval = time.Minute
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(discardWriter{}, &slog.HandlerOptions{Level: slog.LevelError}))
	}
	if source == nil {
		source = func(_ context.Context, _ *SLO) (float64, error) {
			return 0, errors.New("slo: no source registered")
		}
	}
	return &Evaluator{
		repo:     repo,
		source:   source,
		logger:   logger,
		interval: interval,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// Start launches the evaluation loop. The loop exits when ctx is
// cancelled. Errors from the source or the breach callback are
// logged but do not stop the loop.
func (e *Evaluator) Start(ctx context.Context, onBreach BreachFunc) {
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	// Run once immediately so the first evaluation does not wait
	// a full interval.
	e.tick(ctx, onBreach)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.tick(ctx, onBreach)
		}
	}
}

func (e *Evaluator) tick(ctx context.Context, onBreach BreachFunc) {
	e.mu.Lock()
	e.lastRun = e.now()
	e.mu.Unlock()

	if e.repo == nil {
		return
	}
	slos, err := e.repo.ListSLOs(ctx, "")
	if err != nil {
		e.logger.Warn("slo evaluator: list failed", "err", err)
		return
	}
	for _, s := range slos {
		if err := s.Validate(); err != nil {
			e.logger.Warn("slo evaluator: invalid", "id", s.ID, "err", err)
			continue
		}
		actual, err := e.source(ctx, s)
		if err != nil {
			e.logger.Warn("slo evaluator: source error", "id", s.ID, "err", err)
			continue
		}
		if err := s.Evaluate(actual); err == nil {
			continue
		}
		if onBreach == nil {
			continue
		}
		be := BreachEvent{
			SLO:        s,
			Actual:     actual,
			BurnRate:   s.Goal.BurnRate(actual),
			ObservedAt: e.now(),
		}
		if err := onBreach(ctx, be); err != nil {
			e.logger.Warn("slo evaluator: breach callback failed", "id", s.ID, "err", err)
		}
	}
}

// LastRun returns the timestamp of the most recent tick. Useful
// for the /ready handler when an SLO alert is wired to liveness.
func (e *Evaluator) LastRun() time.Time {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastRun
}

type discardWriter struct{}

func (discardWriter) Write(b []byte) (int, error) { return len(b), nil }
