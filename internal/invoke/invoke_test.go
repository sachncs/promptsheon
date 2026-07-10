package invoke

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/approval"
	"github.com/sachncs/promptsheon/internal/budget"
	"github.com/sachncs/promptsheon/internal/capability"
	"github.com/sachncs/promptsheon/internal/eventbus"
	"github.com/sachncs/promptsheon/internal/executor"
	"github.com/sachncs/promptsheon/internal/observation"
	"github.com/sachncs/promptsheon/internal/optimizer/rules"
	"github.com/sachncs/promptsheon/internal/quota"
)

type fakeBus struct{}

func (fakeBus) Subscribe(_ eventbus.Handler, _ ...capability.EventType) (eventbus.Subscription, error) {
	return nil, nil
}
func (fakeBus) Publish(_ capability.Event) error { return nil }

func TestInvokeHappyPath(t *testing.T) {
	t.Parallel()
	now := func() time.Time { return time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC) }
	enforcer := NewDefaultEnforcer(now)
	agg := observation.NewAggregator()
	exec := executor.New(nil, func(_ context.Context, _ executor.InvokeRequest) (executor.InvokeResult, error) {
		return executor.InvokeResult{Output: json.RawMessage(`{"ok":true}`), Status: "ok", PromptTokens: 10, OutputTokens: 5, CostUSDMicro: 100_000, LatencyMS: 50}, nil
	})
	inv := New(enforcer, agg, exec)

	rec, err := inv.Invoke(context.Background(), executor.InvokeRequest{
		WorkspaceID: "ws", ReleaseID: "rel-1", ManifestHash: "x", InputHash: "y", Model: "m", ModelRevision: "r", Input: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if rec.Status != "ok" {
		t.Fatalf("expected ok, got %s", rec.Status)
	}
}

func TestInvokeRejectsQuota(t *testing.T) {
	t.Parallel()
	now := func() time.Time { return time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC) }
	enforcer := NewDefaultEnforcer(now)
	q, err := quota.New(quota.ScopeWorkspace, "ws", quota.WindowSecond, 1, now(), "alice")
	if err != nil {
		t.Fatalf("quota.New: %v", err)
	}
	enforcer.SetQuota(q)
	// First call burns the only slot.
	if err := enforcer.EnforceQuota(context.Background(), "ws"); err != nil {
		t.Fatalf("first call should pass: %v", err)
	}

	agg := observation.NewAggregator()
	exec := executor.New(nil, func(_ context.Context, _ executor.InvokeRequest) (executor.InvokeResult, error) {
		return executor.InvokeResult{Status: "ok"}, nil
	})
	inv := New(enforcer, agg, exec)

	_, err = inv.Invoke(context.Background(), executor.InvokeRequest{
		WorkspaceID: "ws", ReleaseID: "rel-1", Input: json.RawMessage(`{}`),
	})
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("expected ErrQuotaExceeded, got %v", err)
	}
}

func TestInvokeRejectsBudget(t *testing.T) {
	t.Parallel()
	now := func() time.Time { return time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC) }
	enforcer := NewDefaultEnforcer(now)
	b, _ := budget.New(budget.ScopeWorkspace, "ws", budget.PeriodDaily, 0.0001, now(), "alice")
	enforcer.SetBudget(b)

	agg := observation.NewAggregator()
	exec := executor.New(nil, func(_ context.Context, _ executor.InvokeRequest) (executor.InvokeResult, error) {
		return executor.InvokeResult{Status: "ok", CostUSDMicro: 1_000_000}, nil
	})
	inv := New(enforcer, agg, exec)

	rec, err := inv.Invoke(context.Background(), executor.InvokeRequest{
		WorkspaceID: "ws", ReleaseID: "rel-1", Input: json.RawMessage(`{}`),
	})
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("expected ErrBudgetExceeded, got %v", err)
	}
	if rec.Status != "budget_exceeded" {
		t.Fatalf("expected budget_exceeded, got %s", rec.Status)
	}
}

func TestDefaultEnforcerAllowsWhenNoPolicySet(t *testing.T) {
	t.Parallel()
	e := NewDefaultEnforcer(nil)
	if err := e.EnforceBudget(context.Background(), "ws", 1.0); err != nil {
		t.Fatalf("expected no error without budget, got %v", err)
	}
	if err := e.EnforceQuota(context.Background(), "ws"); err != nil {
		t.Fatalf("expected no error without quota, got %v", err)
	}
}

// Compile-time guard that Aggregator implements AggregatorConsumer.
var _ AggregatorConsumer = (*observation.Aggregator)(nil)

// Sentinel: the rules package is the entry-point for v1
// recommendations. We import it for completeness so a later
// refactor that moves rules to internal/optimizer does not lose
// the read-side dependency.
var _ = rules.NewEngine

// Sentinel: approval is wired up in production through the
// release.ApproveWith path (commit 3a4d4d0). The Invoker does not
// touch it today; the alias here documents the dependency so a
// reviewer can follow the chain.
var _ = approval.Approval{}
