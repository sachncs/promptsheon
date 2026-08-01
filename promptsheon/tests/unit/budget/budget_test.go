//go:build tests_migration


package budget_test

import (
	"errors"
	"testing"
	"time"

	. "github.com/sachncs/promptsheon/promptsheon/budget"
	"github.com/sachncs/promptsheon/promptsheon/errs"
)

func TestNewRejectsNonPositiveCap(t *testing.T) {
	t.Parallel()
	if _, err := New(ScopeWorkspace, "ws", PeriodDaily, 0, time.Now(), "alice"); !errors.Is(err, errs.ErrBudgetInvalid) {
		t.Fatalf("expected errs.ErrBudgetInvalid, got %v", err)
	}
}

func TestNewRejectsUnknownScope(t *testing.T) {
	t.Parallel()
	if _, err := New(Scope("bogus"), "ws", PeriodDaily, 100, time.Now(), "alice"); err == nil {
		t.Fatalf("expected error for unknown scope")
	}
}

func TestNewRejectsUnknownPeriod(t *testing.T) {
	t.Parallel()
	if _, err := New(ScopeWorkspace, "ws", Period("bogus"), 100, time.Now(), "alice"); err == nil {
		t.Fatalf("expected error for unknown period")
	}
}

func TestChargeWithinCapAdvances(t *testing.T) {
	t.Parallel()
	b, _ := New(ScopeWorkspace, "ws", PeriodDaily, 100, time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC), "alice")
	got, err := b.Charge(40, b.PeriodStart.Add(time.Hour))
	if err != nil {
		t.Fatalf("charge: %v", err)
	}
	if got.SpentUSD != 40 {
		t.Fatalf("expected spent=40, got %.2f", got.SpentUSD)
	}
	if got.Remaining() != 60 {
		t.Fatalf("expected remaining=60, got %.2f", got.Remaining())
	}
}

func TestChargeExceedingCapRejected(t *testing.T) {
	t.Parallel()
	b, _ := New(ScopeWorkspace, "ws", PeriodDaily, 100, time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC), "alice")
	_, err := b.Charge(200, time.Now())
	if !errors.Is(err, errs.ErrBudget) {
		t.Fatalf("expected errs.ErrBudget, got %v", err)
	}
}

func TestChargeRollsPeriodForward(t *testing.T) {
	t.Parallel()
	b, _ := New(ScopeWorkspace, "ws", PeriodDaily, 100, time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC), "alice")
	b, _ = b.Charge(40, b.PeriodStart.Add(time.Hour))
	// next day
	got, err := b.Charge(30, b.PeriodStart.Add(48*time.Hour))
	if err != nil {
		t.Fatalf("charge (next day): %v", err)
	}
	if got.SpentUSD != 30 {
		t.Fatalf("expected spend to reset, got %.2f", got.SpentUSD)
	}
}

func TestChargeZeroAmount(t *testing.T) {
	t.Parallel()
	b, _ := New(ScopeWorkspace, "ws", PeriodDaily, 100, time.Now(), "alice")
	if _, err := b.Charge(0, time.Now()); err != nil {
		t.Fatalf("zero charge should be allowed: %v", err)
	}
	if _, err := b.Charge(-1, time.Now()); err == nil {
		t.Fatalf("negative charge should be rejected")
	}
}
