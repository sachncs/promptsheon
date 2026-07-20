package main

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// seedAuditDB creates a fresh audit_entries table with a handful
// of pre-048a rows. The schema is minimal: just the columns the
// backfill needs to operate.
func seedAuditDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "audit.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		CREATE TABLE audit_entries (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			action TEXT NOT NULL,
			resource TEXT NOT NULL,
			details TEXT,
			timestamp DATETIME NOT NULL,
			previous_hash TEXT,
			entry_hash TEXT,
			timestamp_str TEXT NOT NULL DEFAULT '',
			resource_kind TEXT NOT NULL DEFAULT '',
			resource_id   TEXT NOT NULL DEFAULT ''
		)`); err != nil {
		t.Fatal(err)
	}
	for i, r := range []struct{ id, user, action, res string }{
		{"e1", "u1", "create", "workspace:abc"},
		{"e2", "u2", "create", "release:r1"},
		{"e3", "u1", "delete", "user:u2"},
		{"e4", "u3", "noop", "no_colon_here"},
		{"e5", "u1", "create", "workspace:xyz"},
		{"e6", "u2", "delete", ""}, // empty resource → empty kind/id
		{"e7", "u1", "update", "release:r1"}, // r1 will be re-backfilled
	} {
		if _, err := db.Exec(
			`INSERT INTO audit_entries (id, user_id, action, resource, timestamp, timestamp_str) VALUES (?, ?, ?, ?, '2024-01-01', '')`,
			r.id, r.user, r.action, r.res,
		); err != nil {
			t.Fatalf("seed %s: %v", r.id, err)
		}
		_ = i
	}
	return db
}

// quietLogger discards all log output. The tests don't assert on
// log content; they assert on the data.
func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

// TestBackfill_PopulatesKindAndID runs the backfill on a small
// DB and asserts every row's resource_kind / resource_id matches
// the legacy split-on-':' rule.
func TestBackfill_PopulatesKindAndID(t *testing.T) {
	if os.Getenv("TEST_BACKFILL") != "1" {
		t.Skip("set TEST_BACKFILL=1 to run")
	}
	db := seedAuditDB(t)
	defer db.Close()

	cfg := defaultConfig()
	cfg.BatchSize = 100
	res, err := backfill(context.Background(), db, cfg, quietLogger())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if res.CancelledEarly {
		t.Fatalf("backfill cancelled early")
	}
	if res.RowsUpdated != 7 {
		t.Errorf("rows updated = %d, want 7", res.RowsUpdated)
	}
	if res.Batches != 1 {
		t.Errorf("batches = %d, want 1 (single batch fits all 7 rows)", res.Batches)
	}

	// Verify every row's kind + id.
	want := map[string][2]string{
		"e1": {"workspace", "abc"},
		"e2": {"release", "r1"},
		"e3": {"user", "u2"},
		"e4": {"", ""},
		"e5": {"workspace", "xyz"},
		"e6": {"", ""},
		"e7": {"release", "r1"},
	}
	rows, _ := db.Query(`SELECT id, resource_kind, resource_id FROM audit_entries ORDER BY id`)
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, k, r string
		if err := rows.Scan(&id, &k, &r); err != nil {
			t.Fatal(err)
		}
		w, ok := want[id]
		if !ok {
			t.Errorf("unexpected id %s", id)
			continue
		}
		if k != w[0] || r != w[1] {
			t.Errorf("%s: got kind=%q id=%q, want %q/%q", id, k, r, w[0], w[1])
		}
	}
	rows.Close()
}

// TestBackfill_Idempotent re-runs the backfill on a fully
// populated table. Rows without a ':' in the resource (e.g.
// "no_colon_here" and "") have an empty kind+id, so the
// production UPDATE touches them on every run (writing the same
// empty values). The data is unchanged; the test asserts that.
func TestBackfill_Idempotent(t *testing.T) {
	if os.Getenv("TEST_BACKFILL") != "1" {
		t.Skip("set TEST_BACKFILL=1 to run")
	}
	db := seedAuditDB(t)
	defer db.Close()

	// First run: populates.
	cfg := defaultConfig()
	cfg.BatchSize = 100
	if _, err := backfill(context.Background(), db, cfg, quietLogger()); err != nil {
		t.Fatal(err)
	}

	// Snapshot the populated data.
	want := snapshot(t, db)

	// Second run: idempotent on the data (rows without a ':'
	// get re-touched, but the values are unchanged).
	if _, err := backfill(context.Background(), db, cfg, quietLogger()); err != nil {
		t.Fatalf("second backfill: %v", err)
	}

	got := snapshot(t, db)
	if !equalSnapshot(want, got) {
		t.Errorf("second backfill changed data:\nwant=%v\ngot=%v", want, got)
	}
}

// TestBackfill_DryRunLeavesDataUnchanged runs the dry-run path
// and asserts the data is unchanged. The dry-run path also
// short-circuits after one batch (the production code's
// heuristic) but the user's data should be untouched.
func TestBackfill_DryRunLeavesDataUnchanged(t *testing.T) {
	if os.Getenv("TEST_BACKFILL") != "1" {
		t.Skip("set TEST_BACKFILL=1 to run")
	}
	db := seedAuditDB(t)
	defer db.Close()

	// Snapshot before.
	before := snapshot(t, db)

	cfg := defaultConfig()
	cfg.BatchSize = 100
	cfg.DryRun = true
	res, err := backfill(context.Background(), db, cfg, quietLogger())
	if err != nil {
		t.Fatal(err)
	}
	if res.RowsUpdated != 0 {
		t.Errorf("dry-run rows updated = %d, want 0", res.RowsUpdated)
	}

	// Verify the data is identical.
	after := snapshot(t, db)
	if !equalSnapshot(before, after) {
		t.Errorf("dry-run modified data:\nbefore=%v\nafter=%v", before, after)
	}
}

func snapshot(t *testing.T, db *sql.DB) [][]string {
	t.Helper()
	rows, err := db.Query(`SELECT id, resource_kind, resource_id FROM audit_entries ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out [][]string
	for rows.Next() {
		var id, k, r string
		rows.Scan(&id, &k, &r)
		out = append(out, []string{id, k, r})
	}
	return out
}

func equalSnapshot(a, b [][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}
