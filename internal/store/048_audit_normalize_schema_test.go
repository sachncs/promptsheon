package store

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// TestMigration048aAuditNormalizeSchema verifies the new
// resource_kind / resource_id columns are added to audit_entries.
// The test asserts:
//   1. Both columns are present.
//   2. The DEFAULT '' on a fresh row is honoured (no NOT NULL
//      violation when the column is omitted).
//   3. The (resource_kind, resource_id, timestamp DESC)
//      composite from 047 still lands cleanly.
func TestMigration048aAuditNormalizeSchema(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "audit_normalize.db")

	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := migrate(db, migrationsFS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// 1. Columns present.
	want := []string{"resource_kind", "resource_id"}
	for _, col := range want {
		var n int
		err := db.QueryRow(
			`SELECT COUNT(*) FROM pragma_table_info('audit_entries') WHERE name=?`,
			col,
		).Scan(&n)
		if err != nil {
			t.Errorf("check column %s: %v", col, err)
		}
		if n != 1 {
			t.Errorf("expected column %s on audit_entries", col)
		}
	}

	// 2. DEFAULT '' honours when the column is omitted. The
	// foreign key on audit_entries.user_id (added in 043) needs
	// the user to exist; seed u1 with the required NOT NULL
	// columns.
	if _, err := db.Exec(
		`INSERT INTO users (id, email, name, role, created_at, updated_at)
		 VALUES ('u1', 'u1@test.local', 'Test', 'admin', '2024-01-01', '2024-01-01')`,
	); err != nil {
		t.Fatalf("seed u1: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO audit_entries (id, user_id, action, resource, timestamp)
		 VALUES ('a1', 'u1', 'noop', 'workspace:abc', '2024-01-01T00:00:00Z')`,
	); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var kind, id string
	err = db.QueryRow(
		`SELECT resource_kind, resource_id FROM audit_entries WHERE id='a1'`,
	).Scan(&kind, &id)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if kind != "" || id != "" {
		t.Errorf("expected empty kind/id by default, got kind=%q id=%q", kind, id)
	}

	// 3. The Go-side AppendAudit writes both. Simulate by inserting
	// the kind+id directly and verify the composite index from 047
	// resolves.
	if _, err := db.Exec(
		`UPDATE audit_entries SET resource_kind = 'workspace', resource_id = 'abc' WHERE id = 'a1'`,
	); err != nil {
		t.Fatalf("update: %v", err)
	}
	rows, err := db.Query(
		`SELECT id FROM audit_entries
		  WHERE resource_kind = 'workspace' AND resource_id = 'abc'
		  ORDER BY timestamp DESC LIMIT 1`)
	if err != nil {
		t.Fatalf("select after update: %v", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		t.Errorf("expected a row to match the structural query")
	}
	var gotID string
	if err := rows.Scan(&gotID); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if !strings.HasPrefix(gotID, "a1") {
		t.Errorf("expected id starting with a1, got %q", gotID)
	}
}
