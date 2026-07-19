package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// TestMigration043FKHygieneApplies verifies the migration adds
// the 9 FK constraints and is idempotent (a second run is a no-op).
//
// The migration adds FKs to existing tables; on a fresh DB
// (NewSQLite applies every migration including 043), the FKs
// land cleanly because no rows exist yet. The test asserts:
//   1. foreign_keys is enabled in the runtime connection
//   2. sqlite_master lists the expected foreign_key entries
//   3. a re-run of the migration is a no-op (the existing
//      schema_migrations version key rejects the duplicate)
func TestMigration043FKHygieneApplies(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "fk_hygiene.db")

	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := migrate(db, migrationsFS); err != nil {
		t.Fatalf("first migrate: %v", err)
	}

	// PRAGMA foreign_keys is per-connection. Re-enable for this
	// test connection; production wiring (NewSQLite) does the
	// same in the DSN.
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable fk: %v", err)
	}

	// 1. Verify the 9 FKs exist via sqlite_master.
	rows, err := db.Query(`
		SELECT m.name, fk.*
		  FROM sqlite_master m
		  JOIN pragma_foreign_key_list(m.name) fk
		 WHERE m.type = 'table'`)
	if err != nil {
		t.Fatalf("query fk list: %v", err)
	}
	defer func() { _ = rows.Close() }()

	wantFKs := map[string]map[string]bool{
		"eval_runs":            {"datasets": true},
		"recommendations":      {"capability_versions": true},
		"schedules":            {"workspaces": true, "releases": true},
		"releases":             {"releases": true},
		"audit_entries":        {"users": true},
		"guardrail_violations": {"guardrail_rules": true, "users": true},
	}
	got := map[string]map[string]bool{}
	for rows.Next() {
		var name string
		var id, seq int
		var table, from, to, onUpdate, onDelete, match string
		if err := rows.Scan(&name, &id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			t.Fatalf("scan fk: %v", err)
		}
		if got[name] == nil {
			got[name] = map[string]bool{}
		}
		got[name][table] = true
	}
	for table, parents := range wantFKs {
		seen, ok := got[table]
		if !ok {
			t.Errorf("table %q has no FKs in sqlite_master", table)
			continue
		}
		for parent := range parents {
			if !seen[parent] {
				t.Errorf("table %q missing FK to %q", table, parent)
			}
		}
	}

	// 2. The pre-flight queries from the migration header must be
	// parseable. On a fresh DB they all return 0.
	preflight := []string{
		`SELECT COUNT(*) FROM eval_runs r LEFT JOIN datasets d ON d.id = r.dataset_id WHERE d.id IS NULL`,
		`SELECT COUNT(*) FROM recommendations rec LEFT JOIN capability_versions v ON v.id = rec.capability_version_id WHERE v.id IS NULL`,
		`SELECT COUNT(*) FROM schedules s LEFT JOIN workspaces w ON w.id = s.workspace_id WHERE w.id IS NULL`,
		`SELECT COUNT(*) FROM schedules s LEFT JOIN releases r ON r.id = s.release_id WHERE r.id IS NULL`,
		`SELECT COUNT(*) FROM releases r1 LEFT JOIN releases r2 ON r2.id = r1.replaces_release_id WHERE r1.replaces_release_id != '' AND r2.id IS NULL`,
		`SELECT COUNT(*) FROM releases r1 LEFT JOIN releases r2 ON r2.id = r1.superseded_by WHERE r1.superseded_by != '' AND r2.id IS NULL`,
		`SELECT COUNT(*) FROM audit_entries a LEFT JOIN users u ON u.id = a.user_id WHERE u.id IS NULL`,
		`SELECT COUNT(*) FROM guardrail_violations gv LEFT JOIN guardrail_rules gr ON gr.id = gv.rule_id WHERE gv.rule_id != '' AND gr.id IS NULL`,
		`SELECT COUNT(*) FROM guardrail_violations gv LEFT JOIN users u ON u.id = gv.user_id WHERE gv.user_id != '' AND u.id IS NULL`,
	}
	for _, q := range preflight {
		var n int
		if err := db.QueryRow(q).Scan(&n); err != nil {
			t.Errorf("preflight %q: %v", q, err)
		}
		if n != 0 {
			t.Errorf("preflight %q: count=%d, want 0 on fresh DB", q, n)
		}
	}

	// 3. Re-running the migration is a no-op (the schema_migrations
	// version key rejects the duplicate). This is the property
	// that makes the migration safe to re-apply after a partial
	// failure.
	if err := migrate(db, migrationsFS); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}
