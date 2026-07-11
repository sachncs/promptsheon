package rollups

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/budget"
	"github.com/sachncs/promptsheon/internal/quota"
)

type fakeBudgetRepo struct {
	items []*budget.Budget
}

func (f *fakeBudgetRepo) CreateBudget(_ context.Context, b *budget.Budget) error {
	f.items = append(f.items, b)
	return nil
}
func (f *fakeBudgetRepo) GetBudget(_ context.Context, id string) (*budget.Budget, error) {
	for _, b := range f.items {
		if b.ID == id {
			return b, nil
		}
	}
	return nil, errors.New("not found")
}
func (f *fakeBudgetRepo) ListBudgetsForTarget(_ context.Context, _ string) ([]*budget.Budget, error) {
	return f.items, nil
}
func (f *fakeBudgetRepo) UpdateBudget(_ context.Context, b *budget.Budget) error {
	for i, existing := range f.items {
		if existing.ID == b.ID {
			f.items[i] = b
			return nil
		}
	}
	return errors.New("not found")
}
func (f *fakeBudgetRepo) DeleteBudget(_ context.Context, id string) error {
	for i, b := range f.items {
		if b.ID == id {
			f.items = append(f.items[:i], f.items[i+1:]...)
			return nil
		}
	}
	return errors.New("not found")
}

type fakeQuotaRepo struct {
	items []*quota.Quota
}

func (f *fakeQuotaRepo) CreateQuota(_ context.Context, q *quota.Quota) error {
	f.items = append(f.items, q)
	return nil
}
func (f *fakeQuotaRepo) GetQuota(_ context.Context, id string) (*quota.Quota, error) {
	for _, q := range f.items {
		if q.ID == id {
			return q, nil
		}
	}
	return nil, errors.New("not found")
}
func (f *fakeQuotaRepo) ListQuotasForTarget(_ context.Context, _ string) ([]*quota.Quota, error) {
	return f.items, nil
}
func (f *fakeQuotaRepo) UpdateQuota(_ context.Context, q *quota.Quota) error {
	for i, existing := range f.items {
		if existing.ID == q.ID {
			f.items[i] = q
			return nil
		}
	}
	return errors.New("not found")
}
func (f *fakeQuotaRepo) DeleteQuota(_ context.Context, id string) error {
	for i, q := range f.items {
		if q.ID == id {
			f.items = append(f.items[:i], f.items[i+1:]...)
			return nil
		}
	}
	return errors.New("not found")
}

func TestAggregatorEmpty(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	a := New(&fakeBudgetRepo{}, &fakeQuotaRepo{})
	got, err := a.BuildWorkspaceSummary(context.Background(), "ws-1", now)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got.WorkspaceID != "ws-1" {
		t.Fatalf("expected ws-1, got %s", got.WorkspaceID)
	}
	if got.OverallHealth != "ok" {
		t.Fatalf("expected ok health for empty workspace, got %s", got.OverallHealth)
	}
}

func TestAggregatorWithBudgetNearLimit(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	b, err := budget.New(budget.ScopeWorkspace, "ws-1", budget.PeriodDaily, 1.0, now, "alice")
	if err != nil {
		t.Fatalf("budget.New: %v", err)
	}
	b, _ = b.Charge(0.95, now)
	brepo := &fakeBudgetRepo{items: []*budget.Budget{&b}}
	a := New(brepo, &fakeQuotaRepo{})
	got, err := a.BuildWorkspaceSummary(context.Background(), "ws-1", now)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got.OverallHealth != "warning" {
		t.Fatalf("expected warning health at 95%% spend, got %s", got.OverallHealth)
	}
	if got.TotalSpendUSD != 0.95 {
		t.Fatalf("expected total 0.95, got %f", got.TotalSpendUSD)
	}
}

func TestAggregatorWithExhaustedQuota(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	q, err := quota.New(quota.ScopeWorkspace, "ws-1", quota.WindowMinute, 1, now, "alice")
	if err != nil {
		t.Fatalf("quota.New: %v", err)
	}
	q, _ = q.Charge(now)
	qrepo := &fakeQuotaRepo{items: []*quota.Quota{&q}}
	a := New(&fakeBudgetRepo{}, qrepo)
	got, err := a.BuildWorkspaceSummary(context.Background(), "ws-1", now)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got.OverallHealth != "degraded" {
		t.Fatalf("expected degraded health with exhausted quota, got %s", got.OverallHealth)
	}
}
