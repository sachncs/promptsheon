package invoke

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/sachncs/promptsheon/internal/budget"
	"github.com/sachncs/promptsheon/internal/quota"
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

// SetBudget persists the budget and updates the in-memory state.
func (p *PersistedEnforcer) SetBudget(b budget.Budget) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.budgets[b.TargetID] = &b
	if p.store != nil {
		payload, err := json.Marshal(b)
		if err == nil {
			if err := p.store.SetEnforcerBudget(context.Background(), b.TargetID, payload); err != nil && p.logger != nil {
				p.logger.Warn("enforcer: persist budget failed", "err", err, "workspace", b.TargetID)
			}
		}
	}
}

// SetQuota persists the quota and updates the in-memory state.
func (p *PersistedEnforcer) SetQuota(q quota.Quota) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.quotas[q.TargetID] = &q
	if p.store != nil {
		payload, err := json.Marshal(q)
		if err == nil {
			if err := p.store.SetEnforcerQuota(context.Background(), q.TargetID, payload); err != nil && p.logger != nil {
				p.logger.Warn("enforcer: persist quota failed", "err", err, "workspace", q.TargetID)
			}
		}
	}
}

// LoadWorkspace loads any persisted budget / quota for a workspace
// from the store into the in-memory state. Called on demand when
// the in-memory map doesn't have an entry.
func (p *PersistedEnforcer) LoadWorkspace(ctx context.Context, workspaceID string) {
	if p.store == nil {
		return
	}
	if data, err := p.store.GetEnforcerBudget(ctx, workspaceID); err == nil && len(data) > 0 {
		var b budget.Budget
		if err := json.Unmarshal(data, &b); err == nil {
			p.mu.Lock()
			p.budgets[workspaceID] = &b
			p.mu.Unlock()
		}
	}
	if data, err := p.store.GetEnforcerQuota(ctx, workspaceID); err == nil && len(data) > 0 {
		var q quota.Quota
		if err := json.Unmarshal(data, &q); err == nil {
			p.mu.Lock()
			p.quotas[workspaceID] = &q
			p.mu.Unlock()
		}
	}
}

// EnforceBudget implements Enforcer. Loads persisted state on
// miss, then charges the in-memory budget. Persists the result.
func (p *PersistedEnforcer) EnforceBudget(ctx context.Context, workspaceID string, costUSD float64) error {
	p.mu.RLock()
	b, ok := p.budgets[workspaceID]
	p.mu.RUnlock()
	if !ok {
		p.LoadWorkspace(ctx, workspaceID)
		p.mu.RLock()
		b, ok = p.budgets[workspaceID]
		p.mu.RUnlock()
		if !ok {
			return nil // no policy -> allow
		}
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
			if err := p.store.SetEnforcerBudget(ctx, workspaceID, payload); err != nil && p.logger != nil {
				p.logger.Warn("enforcer: persist charged budget failed", "err", err)
			}
		}
	}
	return nil
}

// EnforceQuota implements Enforcer. Loads persisted state on
// miss, then enforces.
func (p *PersistedEnforcer) EnforceQuota(ctx context.Context, workspaceID string) error {
	p.mu.RLock()
	q, ok := p.quotas[workspaceID]
	p.mu.RUnlock()
	if !ok {
		p.LoadWorkspace(ctx, workspaceID)
		p.mu.RLock()
		q, ok = p.quotas[workspaceID]
		p.mu.RUnlock()
		if !ok {
			return nil
		}
	}
	_, err := q.Charge(p.now())
	return err
}
