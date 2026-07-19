package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// TestMigration047CoveringIndexes verifies the 9 new composite
// indexes land cleanly. The test asserts each index exists in
// sqlite_master; it does not measure query plans (which would
// require EXPLAIN QUERY PLAN, fragile across SQLite versions).
func TestMigration047CoveringIndexes(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "idx_covering.db")

	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := migrate(db, migrationsFS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	want := []string{
		"idx_schedules_enabled_due",
		"idx_executions_version_recent",
		"idx_versions_capability_version_desc",
		"idx_releases_capability_recent",
		"idx_alerts_rule_recent",
		"idx_datasets_capability_recent",
		"idx_preconditions_capability_created",
		"idx_eval_runs_release_started",
		"idx_audit_user_time",
	}
	for _, name := range want {
		var n int
		err := db.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`,
			name,
		).Scan(&n)
		if err != nil {
			t.Errorf("check %q: %v", name, err)
		}
		if n != 1 {
			t.Errorf("expected index %q to exist, count=%d", name, n)
		}
	}

	// The redundant idx_audit_resource and idx_audit_user are gone.
	for _, name := range []string{"idx_audit_resource", "idx_audit_user"} {
		var n int
		err := db.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`,
			name,
		).Scan(&n)
		if err != nil {
			t.Errorf("check %q: %v", name, err)
		}
		if n != 0 {
			t.Errorf("expected index %q to be dropped, count=%d", name, n)
		}
	}
}
