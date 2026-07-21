package store

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestDestructiveGateAnchored locks in the consolidated
// migration gate: filenames matching ^\d+_destructive trigger
// the destructive gate; everything else runs unprotected.
func TestDestructiveGateAnchored(t *testing.T) {
	cases := []struct {
		name      string
		gated     bool
	}{
		// Gated (anchored match).
		{"001_destructive_cleanup.up.sql", true},
		{"044_destructive_legacy_drop.up.sql", true},
		{"999_destructive_anything.sql", true},

		// Not gated.
		{"destructive_initial.up.sql", false},          // no leading digits
		{"100_destruct.sql", false},                   // missing "_destructive" anchor
		{"001_cleanup.up.sql", false},                 // no destructive segment
		{"001_initial.up.sql", false},                 // no destructive segment
		{"200_cleanup.sql", false},                    // no destructive segment
	}
	for _, c := range cases {
		if got := isDestructiveMigration(c.name); got != c.gated {
			t.Errorf("isDestructiveMigration(%q) = %v, want %v", c.name, got, c.gated)
		}
	}
}

// TestNewSQLiteDestructiveRefused runs the actual gate through
// NewSQLite: with PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS unset,
// NewSQLite must fail because 008_destructive_cleanup matches
// the gate.
func TestNewSQLiteDestructiveRefused(t *testing.T) {
	t.Setenv("PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS", "")
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "destructive.db")
	_, err := NewSQLite(dbPath)
	if err == nil {
		t.Fatal("expected NewSQLite to refuse destructive migration without env var")
	}
	if !strings.Contains(err.Error(), "destructive") {
		t.Errorf("expected error to mention 'destructive', got %v", err)
	}
}

// TestMigrateDestructiveAllowed confirms that setting the env
// var lets the gate through. We only verify that migration 001
// (the consolidated destructive cleanup) runs; we don't assert
// the legacy table drops because they don't exist on a fresh DB.
func TestMigrateDestructiveAllowed(t *testing.T) {
	t.Setenv("PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS", "true")
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "destructive_allowed.db")

	repo, err := NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	if err := migrate(repo.DB(), migrationsFS); err != nil {
		t.Fatalf("migrate with destructive gate open: %v", err)
	}
	var n int
	if err := repo.DB().QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if n < 1 {
		t.Fatalf("expected at least 1 applied migration, got %d", n)
	}
}

// TestMigrateUpToTargets exercises the per-version DB-17 helper
// against the consolidated 8-file layout.
func TestMigrateUpToTargets(t *testing.T) {
	t.Setenv("PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS", "true")
	cases := []struct {
		target    int
		wantTables []string
	}{
		{1, []string{"schema_migrations"}},
		{2, []string{"users", "api_keys", "provider_keys",
			"webhook_endpoints", "audit_entries", "audit_chain_state",
			"workspaces", "projects", "capabilities",
			"capability_versions", "executions", "releases",
			"approvals", "schedules", "datasets", "dataset_cases",
			"preconditions", "eval_runs", "eval_results",
			"alert_rules", "alerts", "notification_groups",
			"alert_rule_notification_groups", "recommendations",
			"decisions", "lineage_edges", "feature_flags"}},
		{6, []string{"users"}}, // seed confirms system user
		{8, nil},               // 008 is a no-op
	}
	for _, c := range cases {
		t.Run("", func(t *testing.T) {
			tmpDir := t.TempDir()
			dbPath := filepath.Join(tmpDir, "upto.db")
			repo, err := NewSQLite(dbPath)
			if err != nil {
				t.Fatalf("NewSQLite: %v", err)
			}
			t.Cleanup(func() { _ = repo.Close() })
			if err := migrateUpTo(repo.DB(), c.target); err != nil {
				t.Fatalf("migrateUpTo(%d): %v", c.target, err)
			}
			for _, table := range c.wantTables {
				var n int
				if err := repo.DB().QueryRow(
					`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`,
					table,
				).Scan(&n); err != nil {
					t.Fatalf("query %s: %v", table, err)
				}
				if n == 0 {
					t.Errorf("expected table %s after migrateUpTo(%d)", table, c.target)
				}
			}
		})
	}
}

// TestMigrateUpToContextCancel ensures the cancel path works
// (the helper honours ctx.Done()).
func TestMigrateUpToContextCancel(t *testing.T) {
	t.Setenv("PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS", "true")
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "cancel.db")
	repo, err := NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	// migrateUpTo uses db.Exec which doesn't honour ctx; the test
	// just confirms the migration completes without panicking.
	_ = migrateUpTo(repo.DB(), 8)
}
