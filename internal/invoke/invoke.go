// Package invoke owns the canonical entry point for an invocation
// of one Capability Release. It enforces Budget and Quota, wires
// the Executor (internal/executor) into the Observer pipeline
// (internal/observation -> internal/recommendation.Producer), and
// returns a hash-stable ExecutionRecord.
//
// The Invoke path is the single place where Budget.Charge and
// Quota.Charge are consulted. Today the aggregations are in-memory;
// production scale moves them to ClickHouse and the Budget/Quota
// stores to the same backend (Tier 3 follow-on).
package invoke

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sachncs/promptsheon/internal/budget"
	"github.com/sachncs/promptsheon/internal/executor"
	"github.com/sachncs/promptsheon/internal/observation"
	"github.com/sachncs/promptsheon/internal/quota"
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
}

// New constructs an Invoker. The Executor is supplied by the caller
// so production wiring decides what the bus looks like.
func New(enforcer Enforcer, agg AggregatorConsumer, exec *executor.Executor) *Invoker {
	return &Invoker{enforcer: enforcer, agg: agg, exec: exec}
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
	rec, err := i.exec.RunRequest(ctx, req, "prod")
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
type DefaultEnforcer struct {
	mu      sync.Mutex
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

// EnforceBudget implements Enforcer.
func (d *DefaultEnforcer) EnforceBudget(_ context.Context, workspaceID string, costUSD float64) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	b, ok := d.budgets[workspaceID]
	if !ok {
		return nil // no policy -> allow
	}
	updated, err := b.Charge(costUSD, d.now())
	if err != nil {
		return err
	}
	d.budgets[workspaceID] = &updated
	return nil
}

// EnforceQuota implements Enforcer.
func (d *DefaultEnforcer) EnforceQuota(_ context.Context, workspaceID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	q, ok := d.quotas[workspaceID]
	if !ok {
		return nil // no policy -> allow
	}
	updated, err := q.Charge(d.now())
	if err != nil {
		return err
	}
	d.quotas[workspaceID] = &updated
	return nil
}

// Verify interface compliance at compile time.
var _ Enforcer = (*DefaultEnforcer)(nil)
var _ AggregatorConsumer = (*observation.Aggregator)(nil)
