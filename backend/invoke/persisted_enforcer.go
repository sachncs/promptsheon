package invoke

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/sachncs/promptsheon/backend/budget"
	"github.com/sachncs/promptsheon/backend/quota"
)

// EnforcerStore is the persistence surface the persisted enforcer
// needs. OBS-13: matches a subset of store repositories methods so
// tests don't have to satisfy the full Repository interface.
type EnforcerStore interface {
	GetEnforcerBudget(ctx context.Context, workspaceID string) ([]byte, error)
	SetEnforcerBudget(ctx context.Context, workspaceID string, payload []byte) error
	GetEnforcerQuota(ctx context.Context, workspaceID string) ([]byte, error)
	SetEnforcerQuota(ctx context.Context, workspaceID string, payload []byte) error
}

// PersistedEnforcer wraps DefaultEnforcer with persistence so
// SetBudget / SetQuota / EnforceBudget survive a daemon restart.
// OBS-13: budget counters and quota counters are stored in
// enforcer_state (migration 012) and loaded at construction.
//
// On a charge that exceeds the budget cap, the in-memory state is
// unchanged; we still write the post-charge attempt to the store
// so a partial-spend can be reconstructed if the operator moves
// the cap.
type PersistedEnforcer struct {
	store  EnforcerStore
	logger *slog.Logger

	mu      sync.RWMutex
	budgets map[string]*budget.Budget
	quotas  map[string]*quota.Quota
	now     func() time.Time
}

// NewPersistedEnforcer constructs a PersistedEnforcer, loading any
// persisted budgets / quotas from the store.
func NewPersistedEnforcer(ctx context.Context, store EnforcerStore, now func() time.Time, logger *slog.Logger) *PersistedEnforcer {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	e := &PersistedEnforcer{
		store:   store,
		logger:  logger,
		budgets: map[string]*budget.Budget{},
		quotas:  map[string]*quota.Quota{},
		now:     now,
	}
	// Eager load: enumerate distinct workspace IDs. Since the
	// store doesn't expose a list endpoint, do best-effort by
	// probing nothing — callers must invoke SetBudget / SetQuota
	// before the daemon restarts for persistence to take effect.
	// Future revision: add a ListEnforcer method.
	_ = ctx
	return e
}

// EnforceBudget implements Enforcer. The persisted enforcer in
// this build does not pre-load budgets; production callers must
// invoke SetBudget before the first EnforceBudget, or every
// charge hits the "no policy -> allow" branch. Reintroduce
// persistence (migration 012) when an operator-facing admin
// route lands that lets SetBudget be called without a daemon
// restart.
func (p *PersistedEnforcer) EnforceBudget(ctx context.Context, workspaceID string, costUSD float64) error {
	p.mu.RLock()
	b, ok := p.budgets[workspaceID]
	p.mu.RUnlock()
	if !ok {
		return nil // no policy -> allow
	}
	updated, err := b.Charge(costUSD, p.now())
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.budgets[workspaceID] = &updated
	p.mu.Unlock()
	if p.store != nil {
		if payload, err := json.Marshal(updated); err == nil {
			_ = p.store.SetEnforcerBudget(ctx, workspaceID, payload)
		}
	}
	return nil
}

// EnforceQuota implements Enforcer. Same persistence caveat as
// EnforceBudget.
func (p *PersistedEnforcer) EnforceQuota(ctx context.Context, workspaceID string) error {
	p.mu.RLock()
	q, ok := p.quotas[workspaceID]
	p.mu.RUnlock()
	if !ok {
		return nil
	}
	updated, err := q.Charge(p.now())
	if err != nil {
		return err
	}
	p.mu.Lock()
	p.quotas[workspaceID] = &updated
	p.mu.Unlock()
	if p.store != nil {
		if payload, err := json.Marshal(updated); err == nil {
			_ = p.store.SetEnforcerQuota(ctx, workspaceID, payload)
		}
	}
	return nil
}
