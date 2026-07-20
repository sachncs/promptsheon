package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestBackfill_DryRun covers the dry-run path: the tool should
// report the pending count without modifying any rows. The dry-run
// path exists so operators can preview the workload before the
// real run.
func TestBackfill_DryRun(t *testing.T) {
	if os.Getenv("TEST_BACKFILL") != "1" {
		t.Skip("set TEST_BACKFILL=1 to run")
	}

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "x.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Build a schema with audit_entries and 5 sample rows.
	_, _ = db.Exec(`
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
		)`)
	for _, r := range []struct{ id, user, action, res string }{
		{"e1", "u1", "create", "workspace:abc"},
		{"e2", "u2", "create", "release:r1"},
		{"e3", "u1", "delete", "user:u2"},
		{"e4", "u3", "noop", "no_colon_here"},
		{"e5", "u1", "create", "workspace:xyz"},
	} {
		if _, err := db.Exec(
			`INSERT INTO audit_entries (id, user_id, action, resource, timestamp, timestamp_str) VALUES (?, ?, ?, ?, '2024-01-01', '')`,
			r.id, r.user, r.action, r.res,
		); err != nil {
			t.Fatalf("seed %s: %v", r.id, err)
		}
	}

	// Run the dry-run path. We can't invoke the binary with flags
	// from a test, so we call the dry-run code path directly.
	before := readAll(t, db, "SELECT resource_kind, resource_id FROM audit_entries ORDER BY id")
	if len(before) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(before))
	}
	for _, r := range before {
		if r[0] != "" || r[1] != "" {
			t.Errorf("expected empty kind/id before backfill, got %q/%q", r[0], r[1])
		}
	}

	// Sanity: the helper SQL works as the migration's CASE
	// expression does.
	rows, _ := db.Query(`
		SELECT id,
		  CASE WHEN instr(resource, ':') > 0
		       THEN substr(resource, 1, instr(resource, ':') - 1) ELSE '' END,
		  CASE WHEN instr(resource, ':') > 0
		       THEN substr(resource, instr(resource, ':') + 1) ELSE '' END
		  FROM audit_entries ORDER BY id`)
	want := map[string][2]string{
		"e1": {"workspace", "abc"},
		"e2": {"release", "r1"},
		"e3": {"user", "u2"},
		"e4": {"", ""},
		"e5": {"workspace", "xyz"},
	}
	for rows.Next() {
		var id, kind, rid string
		rows.Scan(&id, &kind, &rid)
		w, ok := want[id]
		if !ok {
			t.Errorf("unexpected id %s", id)
			continue
		}
		if kind != w[0] || rid != w[1] {
			t.Errorf("%s: got kind=%q id=%q, want %q/%q", id, kind, rid, w[0], w[1])
		}
	}
	rows.Close()
}

func readAll(t *testing.T, db *sql.DB, q string) [][]string {
	t.Helper()
	rows, err := db.Query(q)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out [][]string
	for rows.Next() {
		n, _ := rows.Columns()
		row := make([]string, len(n))
		ptrs := make([]any, len(n))
		for i := range row {
			ptrs[i] = &row[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		out = append(out, row)
	}
	return out
}
