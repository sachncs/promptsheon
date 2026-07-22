package invoke

import (
	"context"
	"testing"
	"time"

	"github.com/sachncs/promptsheon/internal/budget"
)

// fakeEnforcerStore is a minimal in-memory EnforcerStore for tests.
type fakeEnforcerStore struct {
	budgets map[string][]byte
	quotas  map[string][]byte
}

func newFakeEnforcerStore() *fakeEnforcerStore {
	return &fakeEnforcerStore{
		budgets: map[string][]byte{},
		quotas:  map[string][]byte{},
	}
}

func (f *fakeEnforcerStore) GetEnforcerBudget(_ context.Context, id string) ([]byte, error) {
	return f.budgets[id], nil
}
func (f *fakeEnforcerStore) SetEnforcerBudget(_ context.Context, id string, p []byte) error {
	f.budgets[id] = p
	return nil
}
func (f *fakeEnforcerStore) GetEnforcerQuota(_ context.Context, id string) ([]byte, error) {
	return f.quotas[id], nil
}
func (f *fakeEnforcerStore) SetEnforcerQuota(_ context.Context, id string, p []byte) error {
	f.quotas[id] = p
	return nil
}

func TestPersistedEnforcerBudgetRoundTrip(t *testing.T) {
	t.Parallel()
	store := newFakeEnforcerStore()
	e := NewPersistedEnforcer(context.Background(), store, nil, nil)

	b, err := budget.New(budget.ScopeWorkspace, "ws-1", budget.PeriodDaily, 100, time.Now(), "tester")
	if err != nil {
		t.Fatalf("budget.New: %v", err)
	}
	e.SetBudget(b)
	if len(store.budgets) != 1 {
		t.Errorf("expected 1 persisted budget, got %d", len(store.budgets))
	}

	// Charge past the cap. Should reject and the persisted
	// payload should reflect the rejected state.
	if err := e.EnforceBudget(context.Background(), "ws-1", 200); err == nil {
		t.Error("expected budget cap exceeded")
	}

	// Reset by constructing a new enforcer against the same store.
	// The persisted state loads via LoadWorkspace on miss.
	e2 := NewPersistedEnforcer(context.Background(), store, nil, nil)
	if err := e2.EnforceBudget(context.Background(), "ws-1", 30); err != nil {
		t.Errorf("EnforceBudget after reload: %v", err)
	}
}
