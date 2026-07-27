package quota_test

import (
	"github.com/sachncs/promptsheon/backend"
	. "github.com/sachncs/promptsheon/backend/quota"
	"errors"
	"testing"
	"time"
)

func TestNewRejectsNonPositiveLimit(t *testing.T) {
	t.Parallel()
	if _, err := New(ScopeWorkspace, "ws", WindowMinute, 0, time.Now(), "alice"); !errors.Is(err, backend.ErrorQuotaLimitNotPositive) {
		t.Fatalf("expected backend.ErrorQuotaLimitNotPositive, got %v", err)
	}
}

func TestNewRejectsUnknownScope(t *testing.T) {
	t.Parallel()
	if _, err := New(Scope("bogus"), "ws", WindowMinute, 100, time.Now(), "alice"); err == nil {
		t.Fatalf("expected error for unknown scope")
	}
}

func TestNewRejectsUnknownWindow(t *testing.T) {
	t.Parallel()
	if _, err := New(ScopeWorkspace, "ws", Window("bogus"), 100, time.Now(), "alice"); err == nil {
		t.Fatalf("expected error for unknown window")
	}
}

func TestChargeUnderLimitAdvances(t *testing.T) {
	t.Parallel()
	q, _ := New(ScopeWorkspace, "ws", WindowMinute, 3, time.Date(2026, 7, 10, 12, 0, 30, 0, time.UTC), "alice")
	for i := int64(0); i < 3; i++ {
		got, err := q.Charge(time.Date(2026, 7, 10, 12, 0, 30, 0, time.UTC))
		if err != nil {
			t.Fatalf("charge %d: %v", i, err)
		}
		q = got
	}
	if got, err := q.Charge(time.Date(2026, 7, 10, 12, 0, 30, 0, time.UTC)); !errors.Is(err, backend.ErrorQuotaOverLimit) {
		t.Fatalf("expected backend.ErrorQuotaOverLimit, got %v (used=%d)", err, got.Used)
	}
}

func TestChargeResetsWindow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 10, 12, 0, 30, 0, time.UTC)
	q, _ := New(ScopeWorkspace, "ws", WindowSecond, 2, now, "alice")
	q, _ = q.Charge(now)
	q, _ = q.Charge(now)
	if _, err := q.Charge(now); !errors.Is(err, backend.ErrorQuotaOverLimit) {
		t.Fatalf("expected backend.ErrorQuotaOverLimit on third charge in same second")
	}
	next, err := q.Charge(now.Add(time.Second))
	if err != nil {
		t.Fatalf("charge in next window: %v", err)
	}
	if next.Used != 1 {
		t.Fatalf("expected used=1 in new window, got %d", next.Used)
	}
}
