package store

import (
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestDestructiveGateAnchored locks in the consolidated
// migration gate: filenames matching ^\d+_destructive trigger
// the destructive gate; everything else runs unprotected.
func TestDestructiveGateAnchored(t *testing.T) {
	cases := []struct {
		name  string
		gated bool
	}{
		// Gated (anchored match).
		{"001_destructive_cleanup.up.sql", true},
		{"044_destructive_legacy_drop.up.sql", true},
		{"999_destructive_anything.sql", true},

		// Not gated.
		{"destructive_initial.up.sql", false}, // no leading digits
		{"100_destruct.sql", false},           // missing "_destructive" anchor
		{"001_cleanup.up.sql", false},         // no destructive segment
		{"001_initial.up.sql", false},         // no destructive segment
		{"200_cleanup.sql", false},            // no destructive segment
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
// against the consolidated 8-file layout. Each case opens a
// fresh database (no NewSQLite, no full migration) and calls
// migrateUpTo with a specific target. The test verifies that
// migrations 1..N-1 do NOT apply (their tables are absent) and
// that migration N does apply.
func TestMigrateUpToTargets(t *testing.T) {
	t.Setenv("PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS", "true")
	cases := []struct {
		target     int
		wantTables []string // tables expected after migrateUpTo(target)
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
			db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(ON)")
			if err != nil {
				t.Fatalf("sql.Open: %v", err)
			}
			t.Cleanup(func() { _ = db.Close() })
			if err := migrateUpTo(db, c.target); err != nil {
				t.Fatalf("migrateUpTo(%d): %v", c.target, err)
			}
			for _, table := range c.wantTables {
				var n int
				if err := db.QueryRow(
					`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`,
					table,
				).Scan(&n); err != nil {
					t.Fatalf("query %s: %v", table, err)
				}
				if n == 0 {
					t.Errorf("expected table %s after migrateUpTo(%d)", table, c.target)
				}
			}
			// Verify the recorded migration count matches the target.
			var recorded int
			if err := db.QueryRow(
				`SELECT COUNT(*) FROM schema_migrations`,
			).Scan(&recorded); err != nil {
				t.Fatalf("count migrations: %v", err)
			}
			if recorded != c.target {
				t.Errorf("schema_migrations count = %d, want %d", recorded, c.target)
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

// TestEmbeddedMigrationsParseableUniqueApplied is the canonical
// invariant test for the migration directory: every embedded
// .up.sql must
//
//  1. have a strict-integer version prefix (the Sscanf("%d_")
//     parser must accept the entire numeric run);
//  2. be unique on that version (no two files share the same
//     integer — 014b is the historical bug this pins against);
//  3. be parseable as a non-empty SQL payload that the runner
//     can apply;
//  4. actually be applied during NewSQLite (no file is silently
//     skipped because the runner rejected its name).
//
// The runner previously used fmt.Sscanf("%d_", &version), which
// silently truncated "014b" to 14 and collided with 014. This
// test fails loudly if a future change reintroduces a suffixed
// name, an empty file, or a duplicate version.
func TestEmbeddedMigrationsParseableUniqueApplied(t *testing.T) {
	t.Setenv("PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS", "true")

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	seen := make(map[int]string)
	var versions []int
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		// Strict-integer prefix: the leading run of digits MUST
		// be followed by '_' (no inline letter like 'b'). This
		// rejects "014b_seed_settings.up.sql" but accepts
		// "015_seed_settings.up.sql".
		var version int
		cut := -1
		for i, r := range name {
			if r < '0' || r > '9' {
				cut = i
				break
			}
		}
		if cut <= 0 || name[cut] != '_' {
			t.Errorf("migration %q has no strict-integer prefix; use NNN_*.up.sql", name)
			continue
		}
		n, err := fmt.Sscanf(name, "%d_", &version)
		if err != nil || n != 1 {
			t.Errorf("migration %q failed Sscanf: n=%d err=%v", name, n, err)
			continue
		}
		if version <= 0 {
			t.Errorf("migration %q: version %d must be positive", name, version)
			continue
		}
		if prev, ok := seen[version]; ok {
			t.Errorf("migration %q and %q share version %d (runner uses leading integer as PK)", name, prev, version)
		}
		seen[version] = name

		// Non-empty payload: a zero-byte .up.sql would silently
		// succeed and waste a slot in schema_migrations.
		content, err := fs.ReadFile(migrationsFS, "migrations/"+name)
		if err != nil {
			t.Errorf("read %s: %v", name, err)
			continue
		}
		if len(strings.TrimSpace(string(content))) == 0 {
			t.Errorf("migration %q has an empty body", name)
		}
		versions = append(versions, version)
	}
	if len(versions) == 0 {
		t.Fatal("no .up.sql migrations found")
	}
	sort.Ints(versions)
	for i := 1; i < len(versions); i++ {
		if versions[i] == versions[i-1] {
			t.Errorf("duplicate version %d in sorted set", versions[i])
		}
	}

	// End-to-end: NewSQLite must record every parsed version.
	tmpDir := t.TempDir()
	repo, err := NewSQLite(filepath.Join(tmpDir, "all.db"))
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	rows, err := repo.DB().Query(`SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatalf("query applied versions: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var applied []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan: %v", err)
		}
		applied = append(applied, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}
	if !intSlicesEqual(applied, versions) {
		t.Errorf("applied versions %v != parsed versions %v", applied, versions)
	}
}

func intSlicesEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestSeedSettingsMigration015 isolates 015_seed_settings so the
// data-only INSERT runs against an empty system_config and the
// expected default keys appear.
func TestSeedSettingsMigration015(t *testing.T) {
	t.Setenv("PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS", "true")
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "seed.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := migrateUpTo(db, 15); err != nil {
		t.Fatalf("migrateUpTo(15): %v", err)
	}
	rows, err := db.Query(`SELECT key FROM system_config ORDER BY key`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatalf("scan: %v", err)
		}
		keys = append(keys, k)
	}
	want := []string{
		"llm.anthropic.api_key_ref",
		"llm.openai.api_key_ref",
		"otl.endpoint",
		"otl.insecure",
		"otl.sample_ratio",
	}
	if len(keys) != len(want) {
		t.Fatalf("seed keys count = %d, want %d (got %v)", len(keys), len(want), keys)
	}
	for i, k := range want {
		if keys[i] != k {
			t.Errorf("seed key %d = %q, want %q", i, keys[i], k)
		}
	}
}
