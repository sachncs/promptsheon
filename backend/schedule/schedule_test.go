package schedule

import (
	"testing"
	"time"
)

func TestNewRejectsEmptyIDs(t *testing.T) {
	t.Parallel()
	if _, err := New("", "rel", KindCron, "* * * * *", ""); err == nil {
		t.Fatalf("expected error for empty workspace_id")
	}
	if _, err := New("ws", "", KindCron, "* * * * *", ""); err == nil {
		t.Fatalf("expected error for empty release_id")
	}
}

func TestNewRejectsUnknownKind(t *testing.T) {
	t.Parallel()
	if _, err := New("ws", "rel", Kind("bogus"), "", ""); err == nil {
		t.Fatalf("expected error for unknown kind")
	}
}

func TestNextCronKnownExpressions(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		expr string
		want string
	}{
		{"every minute", "* * * * *", "after now"},
		{"every hour at minute 0", "0 * * * *", "next minute=0"},
		{"daily at 03:00", "0 3 * * *", "next 03:00"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			got, err := nextCron(tc.expr, from)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if !got.After(from) {
				t.Fatalf("expected next after %v, got %v", from, got)
			}
		})
	}
}

func TestNextCronRejectsInvalid(t *testing.T) {
	t.Parallel()
	for _, expr := range []string{"", "bad", "60 * * * *", "* 24 * * *", "* * * 13 *", "* * 32 * *", "* * * 7", "* * * * * *"} {
		if _, err := nextCron(expr, time.Now()); err == nil {
			t.Errorf("expected error for %q", expr)
		}
	}
}

func TestMarkFiredAdvancesCountAndNext(t *testing.T) {
	t.Parallel()
	s, err := New("ws", "rel", KindCron, "* * * * *", "")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if s.FiredCount != 0 {
		t.Fatalf("expected initial count 0")
	}
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	s = s.MarkFired(now)
	if s.FiredCount != 1 {
		t.Fatalf("expected count 1, got %d", s.FiredCount)
	}
	if s.LastFireAt == nil || !s.LastFireAt.Equal(now) {
		t.Fatalf("expected LastFireAt = %v, got %v", now, s.LastFireAt)
	}
	if !s.NextFireAt.After(now) {
		t.Fatalf("expected next fire after %v, got %v", now, s.NextFireAt)
	}
}

func TestDisableEnable(t *testing.T) {
	t.Parallel()
	s, _ := New("ws", "rel", KindManual, "", "")
	if !s.Enabled {
		t.Fatalf("expected initial Enabled=true")
	}
	if s.Disable().Enabled {
		t.Fatalf("expected Enabled=false after Disable")
	}
	if !s.Enable().Enabled {
		t.Fatalf("expected Enabled=true after Enable")
	}
}
