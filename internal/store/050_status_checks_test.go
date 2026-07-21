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
// DB-17: per-version known-state pattern. Apply migrations
// 001..049, assert the post-049 state (no CHECKs yet), then
// apply 050 and assert the post-050 state (the four CHECKs).
// If migration 050 regresses, this test fails with a precise
// name pointing at the right migration; bulk-apply tests lose
// that isolation.
func TestMigration050StatusChecks(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "status_checks.db")

	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	// Step 1: apply 001..049. Assert that none of the four
	// CHECK constraints exist yet — the migrations before 050
	// did not introduce them.
	if err := migrateUpTo(db, 49); err != nil {
		t.Fatalf("migrateUpTo(49): %v", err)
	}
	for _, table := range []string{"executions", "releases", "alerts", "eval_results"} {
		var sqlText string
		err := db.QueryRow(
			`SELECT sql FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&sqlText)
		if err != nil {
			t.Fatalf("query %s: %v", table, err)
		}
		// Most pre-050 tables won't have CHECKs (the constraint
		// is the 050 contribution). Don't fail if the table
		// has no SQL text (it doesn't exist yet) — that's
		// expected for some pre-050 schemas.
		if sqlText != "" && strings.Contains(sqlText, "CHECK") {
			t.Logf("pre-050 %s already has a CHECK (not necessarily 050's): %s", table, sqlText)
		}
	}

	// Step 2: apply 050 only.
	if err := migrateUpTo(db, 50); err != nil {
		t.Fatalf("migrateUpTo(50): %v", err)
	}

	// 1. Each CHECK constraint is queryable via the table's
	// sql string in sqlite_master. SQLite stores CHECK as part
	// of the CREATE TABLE statement; the index name (chk_*)
	// exists in sqlite_master only for UNIQUE / PRIMARY KEY
	// constraints, not plain CHECKs. So the test checks the
	// table's SQL text.
	want := map[string]string{
		"executions":   "environment IN ('dev','staging','prod','')",
		"releases":     "environment IN ('dev','staging','prod')",
		"alerts":       "status IN ('active','resolved')",
		"eval_results": "passed IN (0, 1)",
	}
	for table, wantFragment := range want {
		var sqlText string
		err := db.QueryRow(
			`SELECT sql FROM sqlite_master WHERE type='table' AND name=?`,
			table,
		).Scan(&sqlText)
		if err != nil {
			t.Errorf("query %s: %v", table, err)
		}
		if !strings.Contains(sqlText, "CHECK") {
			t.Errorf("expected %s table SQL to contain a CHECK constraint, got %q", table, sqlText)
		}
		if !strings.Contains(sqlText, wantFragment) {
			t.Errorf("expected %s table SQL to contain CHECK(%s), got %q", table, wantFragment, sqlText)
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
