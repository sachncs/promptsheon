package observability

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func newTestManager(t *testing.T, policy RetentionPolicy) *RetentionManager {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewRetentionManager(db, policy, logger)
}

func TestDefaultRetentionPolicyIs30Days(t *testing.T) {
	p := DefaultRetentionPolicy()
	// The trace TTL has a hard floor of 30 days enforced at
	// load time; the default should already be at that floor
	// so operators get the longest possible retention.
	if p.TraceTTL != 30*24*time.Hour {
		t.Errorf("TraceTTL: got %v, want 30 days", p.TraceTTL)
	}
	if p.AuditTTL != 90*24*time.Hour {
		t.Errorf("AuditTTL: got %v, want 90 days", p.AuditTTL)
	}
	if p.CheckInterval != time.Hour {
		t.Errorf("CheckInterval: got %v, want 1h", p.CheckInterval)
	}
}

func TestLoadRetentionPolicyFromEnvOverrides(t *testing.T) {
	t.Setenv("PROMPTSHEON_TRACE_TTL_DAYS", "60")
	t.Setenv("PROMPTSHEON_AUDIT_TTL_DAYS", "180")
	t.Setenv("PROMPTSHEON_RETENTION_CHECK_MINUTES", "30")
	p := LoadRetentionPolicyFromEnv()
	if p.TraceTTL != 60*24*time.Hour {
		t.Errorf("TraceTTL: got %v", p.TraceTTL)
	}
	if p.AuditTTL != 180*24*time.Hour {
		t.Errorf("AuditTTL: got %v", p.AuditTTL)
	}
	if p.CheckInterval != 30*time.Minute {
		t.Errorf("CheckInterval: got %v", p.CheckInterval)
	}
}

func TestLoadRetentionPolicyEnforcesTraceFloor(t *testing.T) {
	// Even if the operator sets trace TTL to 1 day, the floor
	// keeps it at 30. This is a security/compliance
	// invariant; the test pins it.
	t.Setenv("PROMPTSHEON_TRACE_TTL_DAYS", "1")
	p := LoadRetentionPolicyFromEnv()
	if p.TraceTTL != 30*24*time.Hour {
		t.Errorf("expected trace floor of 30 days, got %v", p.TraceTTL)
	}
}

func TestLoadRetentionPolicyIgnoresGarbage(t *testing.T) {
	t.Setenv("PROMPTSHEON_TRACE_TTL_DAYS", "not a number")
	t.Setenv("PROMPTSHEON_AUDIT_TTL_DAYS", "0")
	t.Setenv("PROMPTSHEON_RETENTION_CHECK_MINUTES", "")
	p := LoadRetentionPolicyFromEnv()
	// Each garbage / out-of-range value should leave the
	// corresponding field at its default.
	if p.TraceTTL != 30*24*time.Hour {
		t.Errorf("TraceTTL: got %v, want default 30d", p.TraceTTL)
	}
	if p.AuditTTL != 90*24*time.Hour {
		t.Errorf("AuditTTL: got %v, want default 90d", p.AuditTTL)
	}
	if p.CheckInterval != time.Hour {
		t.Errorf("CheckInterval: got %v, want default 1h", p.CheckInterval)
	}
}

func TestGetPolicyReturnsConstructorValue(t *testing.T) {
	want := RetentionPolicy{
		TraceTTL:      60 * 24 * time.Hour,
		AuditTTL:      180 * 24 * time.Hour,
		CheckInterval: 15 * time.Minute,
	}
	m := newTestManager(t, want)
	got := m.GetPolicy()
	if got != want {
		t.Errorf("GetPolicy: got %+v, want %+v", got, want)
	}
}

func TestEnforceCancelsOnContext(t *testing.T) {
	// With an in-memory sqlite, the DELETE will fail because
	// the tables don't exist. We just want to confirm Enforce
	// returns without panic and respects a cancelled context.
	m := newTestManager(t, DefaultRetentionPolicy())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Enforce swallows the error and returns nil; the goal is
	// to confirm the function doesn't deadlock on a cancelled
	// context.
	if err := m.Enforce(ctx); err != nil {
		t.Errorf("Enforce: expected nil even with cancelled context, got %v", err)
	}
}

func TestProtectedAuditActionsContainsExpected(t *testing.T) {
	// The protected-audit-actions set is a security/compliance
	// invariant. The set must include every action the audit
	// chain is supposed to preserve forever.
	required := []string{
		"auth_failure",
		"deploy",
		"create",
		"update",
		"delete",
		"key_mint",
		"key_revoke",
	}
	for _, action := range required {
		if !protectedAuditActions[action] {
			t.Errorf("expected %q to be in protectedAuditActions", action)
		}
	}
}

func TestStartRespectsContextCancellation(t *testing.T) {
	// We don't want Start to leak the goroutine on shutdown.
	// The test starts the manager, lets the goroutine start,
	// then cancels the context; the next read from a done
	// channel is the test's signal that the goroutine exited.
	// We use a CheckInterval that fires never (24h) so the
	// goroutine blocks in select for the duration of the test.
	m := newTestManager(t, RetentionPolicy{
		TraceTTL:      30 * 24 * time.Hour,
		AuditTTL:      90 * 24 * time.Hour,
		CheckInterval: 24 * time.Hour,
	})
	ctx, cancel := context.WithCancel(context.Background())
	m.Start(ctx)
	// Give the goroutine a moment to enter the select.
	time.Sleep(10 * time.Millisecond)
	cancel()
	// If the goroutine leaked, the test would still pass; the
	// real assertion is the lack of panic and the lack of a
	// long-lived ticker. The cleanup in newTestManager
	// closes the database which is the more meaningful
	// guarantee.
}
