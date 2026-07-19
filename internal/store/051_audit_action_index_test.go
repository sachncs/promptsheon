package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// TestMigration051AuditActionIndex verifies the new composite
// (user_id, action, timestamp DESC) index lands cleanly.
func TestMigration051AuditActionIndex(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "audit_action.db")

	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := migrate(db, migrationsFS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// 1. The new index exists.
	var n int
	err = db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_audit_user_action_time'`,
	).Scan(&n)
	if err != nil {
		t.Errorf("check: %v", err)
	}
	if n != 1 {
		t.Errorf("expected idx_audit_user_action_time to exist, count=%d", n)
	}

	// 2. The existing idx_audit_user_time from 047 is preserved.
	err = db.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_audit_user_time'`,
	).Scan(&n)
	if err != nil {
		t.Errorf("check 047: %v", err)
	}
	if n != 1 {
		t.Errorf("expected idx_audit_user_time to exist, count=%d", n)
	}
}
