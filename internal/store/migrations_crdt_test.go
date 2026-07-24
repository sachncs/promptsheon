// Migration tests for the bandit CRDT (016) and settings
// CRDT (017) schema changes. Each test isolates one migration's
// effect by running migrateUpTo on a fresh DB at the target
// version, then asserting the column shape.
package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestBanditArmCountersMigration016(t *testing.T) {
	t.Setenv("PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS", "true")
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "bandit.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(ON)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	err = migrateUpTo(db, 16)
	if err != nil {
		t.Fatalf("migrateUpTo(16): %v", err)
	}
	rows, err := db.Query(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='bandit_arm_counters'`,
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		t.Fatal("bandit_arm_counters table missing after migration 016")
	}
	// PK conflict path: an INSERT followed by an UPSERT with
	// the same (arm_id, replica_id) must hit the conflict
	// branch.
	if _, err := db.Exec(
		`INSERT INTO bandit_arm_counters (arm_id, replica_id, successes, failures) VALUES ('arm-1', 'rep-a', 5, 2)`,
	); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO bandit_arm_counters (arm_id, replica_id, successes, failures) VALUES ('arm-1', 'rep-a', 6, 3) ON CONFLICT(arm_id, replica_id) DO UPDATE SET successes=excluded.successes, failures=excluded.failures`,
	); err != nil {
		t.Fatalf("conflict update: %v", err)
	}
	var s, f int
	if err := db.QueryRow(`SELECT successes, failures FROM bandit_arm_counters WHERE arm_id='arm-1' AND replica_id='rep-a'`).Scan(&s, &f); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if s != 6 || f != 3 {
		t.Fatalf("upsert: got (%d, %d) want (6, 3)", s, f)
	}
}

func TestSystemConfigCRDTMigration017(t *testing.T) {
	t.Setenv("PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS", "true")
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "settings_crdt.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(ON)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	err = migrateUpTo(db, 17)
	if err != nil {
		t.Fatalf("migrateUpTo(17): %v", err)
	}
	rows, err := db.Query(`SELECT name FROM pragma_table_info('system_config')`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	cols := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		cols[name] = true
	}
	for _, want := range []string{"replica_id", "version_vector", "tombstone", "write_ts"} {
		if !cols[want] {
			t.Fatalf("missing column %q after migration 017", want)
		}
	}
	// The defaults are non-destructive: a legacy row
	// (inserted before the migration) gets sensible CRDT
	// metadata so reads continue to work.
	if _, err := db.Exec(
		`INSERT INTO system_config (key, value, updated_by) VALUES ('legacy.k', '"x"', 'legacy')`,
	); err != nil {
		t.Fatalf("legacy insert: %v", err)
	}
	var tombstone int
	var writeTS int64
	var replicaID, vecJSON string
	if err := db.QueryRow(
		`SELECT tombstone, write_ts, replica_id, version_vector FROM system_config WHERE key='legacy.k'`,
	).Scan(&tombstone, &writeTS, &replicaID, &vecJSON); err != nil {
		t.Fatalf("legacy read: %v", err)
	}
	if tombstone != 0 {
		t.Fatalf("legacy tombstone default: got %d want 0", tombstone)
	}
	if writeTS != 0 {
		t.Fatalf("legacy write_ts default: got %d want 0", writeTS)
	}
	if replicaID != "init" {
		t.Fatalf("legacy replica_id default: got %q want init", replicaID)
	}
	if vecJSON != "{}" {
		t.Fatalf("legacy version_vector default: got %q want {}", vecJSON)
	}
}
