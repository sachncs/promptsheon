// Package invoke owns the canonical entry point for an invocation
// of one Capability Release. It enforces Budget and Quota, wires
// the Executor (internal/executor) into the Observer pipeline
// (internal/observation -> internal/recommendation.Producer), and
// returns a hash-stable ExecutionRecord.
//
// The Invoke path is the single place where Budget.Charge and
// Quota.Charge are consulted. Today the aggregations are in-memory;
// production scale moves them to ClickHouse and the Budget/Quota
// stores to the same backend.
package invoke

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/sachncs/promptsheon/internal/budget"
	"github.com/sachncs/promptsheon/internal/executor"
	"github.com/sachncs/promptsheon/internal/metrics"
	"github.com/sachncs/promptsheon/internal/observation"
	"github.com/sachncs/promptsheon/internal/quota"
	"github.com/sachncs/promptsheon/internal/trace"
)

// Caller is the actual LLM provider invocation. It is the same
// shape the Executor accepts.
type Caller = executor.Caller

// Enforcer is the consumer-defined contract for Budget and Quota
// enforcement. Production wiring calls into the SQLite / Postgres
// Budget/Quota stores; tests pass an in-memory mock.
type Enforcer interface {
	EnforceBudget(ctx context.Context, workspaceID string, costUSD float64) error
	EnforceQuota(ctx context.Context, workspaceID string) error
}

// AggregatorConsumer is satisfied by *observation.Aggregator; it
// accepts execution records so the rule engine can fire on the
// post-invocation observation window.
type AggregatorConsumer interface {
	Add(executor.ExecutionRecord)
}

// ExecutorFactory wires a Caller into a fully wired Executor for one
// invocation. The default implementation wraps Caller and emits
// capability.EventExecutionFinished on the supplied bus.
type ExecutorFactory func(caller Caller) *executor.Executor

// CallerFromBudget is the consumer side of the budget/Caller hook:
// when a Caller reports its expected cost before the call, the
// caller can return the cost up-front and Invoke() consults Budget
// once with the actual cost returned.
type PreCost interface {
	EstimatedCost(req executor.InvokeRequest) (microUSD int64, ok bool)
}

// Invoker is the top-level facade for one Invoke call.
type Invoker struct {
	enforcer Enforcer
	agg      AggregatorConsumer
	exec     *executor.Executor

	// OBS-5a: optional metrics/tracer wiring. When non-nil, every
	// Invoke call goes through metrics.LLMMiddleware which records
	// the per-call latency, success/error counts, and span.
	llmCollector *metrics.Collector
	tracer       trace.Tracer
	logger       *slog.Logger
}

// New constructs an Invoker. The Executor is supplied by the caller
// so production wiring decides what the bus looks like.
func New(enforcer Enforcer, agg AggregatorConsumer, exec *executor.Executor) *Invoker {
	return &Invoker{enforcer: enforcer, agg: agg, exec: exec}
}

// WithObservability wires the metrics collector + tracer used by
// the LLMMiddleware wrapper. OBS-5a. May be called once at
// construction; passing nil values is a no-op.
func (i *Invoker) WithObservability(c *metrics.Collector, t trace.Tracer, l *slog.Logger) *Invoker {
	i.llmCollector = c
	i.tracer = t
	i.logger = l
	return i
}

// Errors returned by Invoke.
var (
	ErrBudgetExceeded = errors.New("invoke: budget exceeded")
	ErrQuotaExceeded  = errors.New("invoke: quota exceeded")
	ErrBudgetEnforcer = errors.New("invoke: budget enforcer error")
	ErrQuotaEnforcer  = errors.New("invoke: quota enforcer error")
)

// Invoke runs one Execute against the Caller and returns the
// resulting ExecutionRecord.
//
// Quota is consulted FIRST (atomic, in-memory) and yields 429 if
// the per-window rate is exhausted. Budget is consulted SECOND
// based on the Caller-reported cost (after the call); a refused
// budget returns 402 Payment Required without persisting the
// cost.
//
// Caller errors are returned verbatim; they are also persisted in
// the ExecutionRecord so the audit chain and Replay buffer see
// them.
func (i *Invoker) Invoke(ctx context.Context, req executor.InvokeRequest) (executor.ExecutionRecord, error) {
	if err := i.enforcer.EnforceQuota(ctx, req.WorkspaceID); err != nil {
		if errors.Is(err, quota.ErrOverLimit) {
			return executor.ExecutionRecord{}, ErrQuotaExceeded
		}
		return executor.ExecutionRecord{}, fmt.Errorf("%w: %w", ErrQuotaEnforcer, err)
	}
	rec, err := i.invokeLLM(ctx, req, "prod")
	if err != nil {
		return rec, err
	}
	if err := i.enforcer.EnforceBudget(ctx, req.WorkspaceID, rec.CostUSD); err != nil {
		if errors.Is(err, budget.ErrCapExceeded) {
			rec.Status = "budget_exceeded"
			i.agg.Add(rec)
			return rec, ErrBudgetExceeded
		}
		i.agg.Add(rec)
		return rec, fmt.Errorf("%w: %w", ErrBudgetEnforcer, err)
	}
	i.agg.Add(rec)
	return rec, nil
}

// -- Default Enforcer --
// DefaultEnforcer is the in-process, in-memory Budget + Quota
// enforcer. Production wiring supplies a Backend-backed one.
//
// PERF-4 follow-up: the enforcer used to hold one sync.Mutex
// across every read and write. Reads (EnforceBudget, EnforceQuota
// for an unknown workspace) were the vast majority; contention
// would have scaled linearly with concurrent invocations. Use
// sync.RWMutex so reads run in parallel and only the
// rare workspace-known write path takes the write lock.
type DefaultEnforcer struct {
	mu      sync.RWMutex
	budgets map[string]*budget.Budget
	quotas  map[string]*quota.Quota
	now     func() time.Time
}

// NewDefaultEnforcer constructs a DefaultEnforcer used by tests and
// single-process installs. now is the clock; the default is
// time.Now.UTC.
func NewDefaultEnforcer(now func() time.Time) *DefaultEnforcer {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &DefaultEnforcer{
		budgets: map[string]*budget.Budget{},
		quotas:  map[string]*quota.Quota{},
		now:     now,
	}
}

// SetBudget registers a Budget for the workspace.
func (d *DefaultEnforcer) SetBudget(b budget.Budget) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.budgets[b.TargetID] = &b
}

// SetQuota registers a Quota for the workspace.
func (d *DefaultEnforcer) SetQuota(q quota.Quota) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.quotas[q.TargetID] = &q
}

// EnforceBudget implements Enforcer. Read-mostly path; takes the
// read lock for the lookup and upgrades to the write lock only
// when a budget is configured and being charged.
func (d *DefaultEnforcer) EnforceBudget(_ context.Context, workspaceID string, costUSD float64) error {
	d.mu.RLock()
	b, ok := d.budgets[workspaceID]
	d.mu.RUnlock()
	if !ok {
		return nil // no policy -> allow
	}
	updated, err := b.Charge(costUSD, d.now())
	if err != nil {
		return err
	}
	d.mu.Lock()
	d.budgets[workspaceID] = &updated
	d.mu.Unlock()
	return nil
}

// EnforceQuota implements Enforcer. Read-mostly path; takes the
// read lock for the lookup and upgrades to the write lock only
// when a quota is configured and being charged.
func (d *DefaultEnforcer) EnforceQuota(_ context.Context, workspaceID string) error {
	d.mu.RLock()
	q, ok := d.quotas[workspaceID]
	d.mu.RUnlock()
	if !ok {
		return nil // no policy -> allow
	}
	updated, err := q.Charge(d.now())
	if err != nil {
		return err
	}
	d.mu.Lock()
	d.quotas[workspaceID] = &updated
	d.mu.Unlock()
	return nil
}

// invokeLLM wraps i.exec.RunRequest in the LLMMiddleware when
// observability is wired. Without observability the call goes
// straight through. OBS-5a: every LLM call now records
// latency, success/error counts, and an OTel span.
func (i *Invoker) invokeLLM(ctx context.Context, req executor.InvokeRequest, env string) (executor.ExecutionRecord, error) {
	if i.llmCollector == nil || i.tracer == nil {
		return i.exec.RunRequest(ctx, req, env)
	}
	wrapped := metrics.LLMMiddleware(i.llmCollector, i.tracer, i.logger)(
		func(operation string, r any) (any, error) {
			return i.exec.RunRequest(ctx, r.(executor.InvokeRequest), operation)
		},
	)
	rec, err := wrapped(env, req)
	if err != nil {
		return executor.ExecutionRecord{}, err
	}
	typed, ok := rec.(executor.ExecutionRecord)
	if !ok {
		return executor.ExecutionRecord{}, fmt.Errorf("invoke: middleware returned %T, want ExecutionRecord", rec)
	}
	return typed, nil
}

// Verify interface compliance at compile time.
var _ Enforcer = (*DefaultEnforcer)(nil)
var _ AggregatorConsumer = (*observation.Aggregator)(nil)
