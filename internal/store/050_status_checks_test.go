package store

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// TestMigration050StatusChecks verifies the 4 CHECK constraints
// land and that the data is clean (no existing rows violate the
// closed enums after the truncation).
//
// Note: schedules has no environment column (cron/webhook
// dispatch is environment-agnostic; the release's environment
// is the discriminator), so the original 5-CHECK plan drops to
// 4. schedules.kind is closed in Go via the Kind* constants; a
// DB-level CHECK for kind is a follow-on.
func TestMigration050StatusChecks(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "status_checks.db")

	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := migrate(db, migrationsFS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// 1. Each CHECK constraint is queryable via the table's
	// sql string in sqlite_master. SQLite stores CHECK as part
	// of the CREATE TABLE statement; the index name (chk_*)
	// exists in sqlite_master only for UNIQUE / PRIMARY KEY
	// constraints, not plain CHECKs. So the test checks the
	// table's SQL text.
	want := map[string]string{
		"executions":   "chk_executions_env",
		"releases":     "chk_releases_env",
		"alerts":       "chk_alert_status",
		"eval_results": "chk_er_passed",
	}
	for table, wantName := range want {
		var sqlText string
		err := db.QueryRow(
			`SELECT sql FROM sqlite_master WHERE type='table' AND name=?`,
			table,
		).Scan(&sqlText)
		if err != nil {
			t.Errorf("query %s: %v", table, err)
		}
		if !strings.Contains(sqlText, wantName) {
			t.Errorf("expected %s table SQL to contain %q, got %q", table, wantName, sqlText)
		}
	}

	// 2. The CHECK actively rejects bad data: try to insert
	// executions.environment='production' (typo) and confirm
	// the INSERT fails. Also alerts.status='superceded'.
	for _, q := range []string{
		`INSERT INTO workspaces (id, name, created_at, updated_at) VALUES ('w1', 't', '2024-01-01', '2024-01-01')`,
		`INSERT INTO projects (id, workspace_id, name, created_at, updated_at) VALUES ('p1', 'w1', 't', '2024-01-01', '2024-01-01')`,
		`INSERT INTO capabilities (id, project_id, name, created_at, updated_at) VALUES ('c1', 'p1', 't', '2024-01-01', '2024-01-01')`,
		`INSERT INTO capability_versions (id, capability_id, version, created_at) VALUES ('v1', 'c1', 1, '2024-01-01')`,
		`INSERT INTO releases (id, capability_id, capability_version, manifest, environment, status, created_by, created_at) VALUES ('r1', 'c1', 1, '{}', 'dev', 'pending', 'u1', '2024-01-01')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
	if _, err := db.Exec(
		`INSERT INTO executions (id, capability_version_id, timestamp, environment) VALUES ('e1', 'v1', '2024-01-01', 'production')`,
	); err == nil {
		t.Errorf("expected CHK violation on executions.environment='production'")
	}
	if _, err := db.Exec(
		`INSERT INTO alerts (id, rule_id, rule_name, severity, status, triggered_at) VALUES ('a1', 'r1', 'rule1', 'high', 'superceded', '2024-01-01')`,
	); err == nil {
		t.Errorf("expected CHK violation on alerts.status='superceded'")
	}
}
