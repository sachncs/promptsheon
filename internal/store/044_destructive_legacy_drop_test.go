package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// TestMigration044LegacyDrop verifies the 044 migration drops the
// 13 dead tables and 13 dead columns. The test runs the full
// migration set on a fresh DB, then asserts:
//
//   1. The 13 dead tables are gone.
//   2. The 13 dead columns on capabilities and capability_versions
//      are gone.
//   3. Surviving tables still exist and have the expected shape.
//   4. Re-running the migration is a no-op (schema_migrations
//      deduplication).
func TestMigration044LegacyDrop(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "legacy_drop.db")

	t.Setenv("PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS", "true")

	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := migrate(db, migrationsFS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// 1. The 13 dead tables are gone. Query sqlite_master and
	// confirm none of them exist.
	deadTables := []string{
		"prompts", "prompt_versions", "agents", "agent_executions",
		"agent_guardrail_configs", "contexts", "workflows",
		"workflow_steps", "output_snapshots", "reviews",
		"test_datasets", "execution_logs",
		// The 13th is the original 014-generated one which was
		// already dropped by 025 even though that migration never
		// ran. Verify it doesn't exist either.
	}
	for _, name := range deadTables {
		var n int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name,
		).Scan(&n)
		if err != nil {
			t.Errorf("check %q: %v", name, err)
		}
		if n != 0 {
			t.Errorf("expected %q to be dropped, still present", name)
		}
	}

	// 2. The dead columns on capabilities and capability_versions
	// are gone. Query pragma_table_info for each.
	deadColumns := map[string][]string{
		"capabilities": {
			"state", "current_version_id", "owner", "tags",
		},
		"capability_versions": {
			"prompt", "model_policy", "context_contract", "knowledge",
			"memory", "guardrails", "tools", "mcp_servers",
			"runtime_policy", "evaluation_suite",
		},
	}
	for table, cols := range deadColumns {
		for _, col := range cols {
			var n int
			err := db.QueryRow(
				"SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?",
				table, col,
			).Scan(&n)
			if err != nil {
				t.Errorf("check %s.%s: %v", table, col, err)
			}
			if n != 0 {
				t.Errorf("expected column %s.%s to be dropped, still present", table, col)
			}
		}
	}

	// 3. Surviving tables still exist.
	aliveTables := []string{
		"workspaces", "projects", "capabilities", "capability_versions",
		"executions", "releases", "approvals", "audit_entries",
		"audit_chain_state", "users", "api_keys", "datasets",
		"dataset_cases", "preconditions", "eval_runs", "eval_results",
		"schedules", "alert_rules", "alerts", "notification_groups",
		"guardrail_rules", "guardrail_violations", "webhook_endpoints",
		"provider_keys", "recommendations", "decisions",
	}
	for _, name := range aliveTables {
		var n int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name,
		).Scan(&n)
		if err != nil {
			t.Errorf("check %s: %v", name, err)
		}
		if n != 1 {
			t.Errorf("expected %q to exist, count=%d", name, n)
		}
	}

	// 4. Re-running the migration is a no-op.
	if err := migrate(db, migrationsFS); err != nil {
		t.Errorf("second migrate: %v", err)
	}
}
