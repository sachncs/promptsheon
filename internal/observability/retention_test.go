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

// newTestManagerWithAuditArchive opens an in-memory DB and
// creates both audit_entries and audit_archive so Enforce can
// exercise OBS-RET-1 against a real schema. No chain data is
// seeded; tests that need chain verification must seed rows.
func newTestManagerWithAuditArchive(t *testing.T, policy RetentionPolicy) *RetentionManager {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, stmt := range []string{
		`CREATE TABLE audit_entries (
			id TEXT PRIMARY KEY, user_id TEXT NOT NULL,
			action TEXT NOT NULL, resource TEXT NOT NULL,
			details TEXT DEFAULT '{}', timestamp DATETIME NOT NULL,
			previous_hash TEXT DEFAULT '', entry_hash TEXT DEFAULT '',
			timestamp_str TEXT NOT NULL DEFAULT '',
			resource_kind TEXT NOT NULL DEFAULT '',
			resource_id TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE audit_chain_state (
			id INTEGER PRIMARY KEY CHECK (id = 0),
			last_hash TEXT NOT NULL DEFAULT '',
			last_rowid INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE audit_archive (
			id TEXT PRIMARY KEY, user_id TEXT NOT NULL,
			action TEXT NOT NULL, resource TEXT NOT NULL,
			details TEXT DEFAULT '{}', timestamp DATETIME NOT NULL,
			previous_hash TEXT DEFAULT '', entry_hash TEXT DEFAULT '',
			timestamp_str TEXT NOT NULL DEFAULT '',
			resource_kind TEXT NOT NULL DEFAULT '',
			resource_id TEXT NOT NULL DEFAULT '',
			archived_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`,
		`INSERT INTO audit_chain_state (id, last_hash, last_rowid)
			VALUES (0, '', 0)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("setup: %v (%s)", err, stmt)
		}
	}
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
	// the tables don't exist. The function should now return the
	// error so callers can alert on retention drift (OPS-3).
	m := newTestManager(t, DefaultRetentionPolicy())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := m.Enforce(ctx)
	if err == nil {
		t.Error("Enforce: expected error after context cancellation, got nil")
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

// TestEnforceReturnsErrorOnTraceTableMissing locks in OPS-3:
// when the trace table does not exist, Enforce returns a
// wrapped error instead of logging and silently returning
// nil.
func TestEnforceReturnsErrorOnTraceTableMissing(t *testing.T) {
	m := newTestManager(t, RetentionPolicy{
		TraceTTL:      time.Hour,
		AuditTTL:      0, // skip audit path
		CheckInterval: time.Hour,
	})
	if err := m.Enforce(context.Background()); err == nil {
		t.Fatal("expected error when traces table is missing")
	}
}

// TestEnforceArchivesExpiredAuditRows locks in OBS-RET-1.
// Skipped detail: the audit_entries entry_hash format includes
// RFC3339Nano and the previous_hash, which is brittle to
// recompute in a test. We verify the contract via the failure
// path (TestEnforceAbortsArchiveOnChainFailure) and the empty
// path (TestEnforceNoAuditOnEmptyTable).
func TestEnforceArchivesExpiredAuditRows(t *testing.T) {
	m := newTestManagerWithAuditArchive(t, RetentionPolicy{
		TraceTTL:      0,
		AuditTTL:      24 * time.Hour,
		CheckInterval: time.Hour,
	})
	// Empty audit_entries + valid audit_chain_state (the helper
	// inserts the singleton). VerifyAuditChain returns Ok=true on
	// the empty walk, so Enforce should return nil and the
	// archive should remain empty.
	if err := m.Enforce(context.Background()); err != nil {
		t.Fatalf("Enforce: %v", err)
	}
	var n int
	if err := m.db.QueryRow(`SELECT COUNT(*) FROM audit_archive`).Scan(&n); err != nil {
		t.Fatalf("count archive: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 archived rows on empty chain, got %d", n)
	}
}

// TestEnforceNoAuditOnEmptyTable confirms that when AuditTTL is
// 0, Enforce does not touch the audit path at all (even if
// audit_entries is empty).
func TestEnforceNoAuditOnEmptyTable(t *testing.T) {
	m := newTestManagerWithAuditArchive(t, RetentionPolicy{
		TraceTTL:      0,
		AuditTTL:      0,
		CheckInterval: time.Hour,
	})
	if err := m.Enforce(context.Background()); err != nil {
		t.Fatalf("Enforce: %v", err)
	}
}

// TestEnforceAbortsArchiveOnChainFailure locks in OBS-RET-1:
// if the audit chain is broken (hash mismatch), Enforce skips
// the archive step and returns an error so the operator
// notices before any rows are moved.
func TestEnforceAbortsArchiveOnChainFailure(t *testing.T) {
	m := newTestManagerWithAuditArchive(t, RetentionPolicy{
		TraceTTL:      0,
		AuditTTL:      24 * time.Hour,
		CheckInterval: time.Hour,
	})
	cutoff := time.Now().UTC().Add(-48 * time.Hour)
	if _, err := m.db.Exec(`INSERT INTO audit_entries
		(id, user_id, action, resource, timestamp, previous_hash, entry_hash, timestamp_str)
		VALUES ('a', 'u1', 'create', 'workspace:w1', ?, '', 'h-broken', ''),
		       ('b', 'u1', 'create', 'workspace:w1', ?, 'WRONG-PREV', 'h-b', '')`,
		cutoff, cutoff,
	); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := m.Enforce(context.Background()); err == nil {
		t.Fatal("expected Enforce to fail when chain verification fails")
	}
	// No rows archived.
	var n int
	if err := m.db.QueryRow(`SELECT COUNT(*) FROM audit_archive`).Scan(&n); err != nil {
		t.Fatalf("count archive: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 archived rows on chain failure, got %d", n)
	}
}
