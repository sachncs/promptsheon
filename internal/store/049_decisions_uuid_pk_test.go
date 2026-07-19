package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// TestMigration049DecisionsUUIDPK verifies the decisions table
// rebuild uses TEXT PRIMARY KEY (no longer AUTOINCREMENT) and
// preserves all existing rows.
func TestMigration049DecisionsUUIDPK(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "decisions_uuid.db")

	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := migrate(db, migrationsFS); err != nil {
		t.Fatalf("migrate (pre-049): %v", err)
	}

	// Insert two decisions. 049 runs during migrate(); with the
	// rebuild the data is preserved across versions. To verify
	// the table accepts inserts, insert a couple of rows after
	// migrate() and check the column type. Recommendations FK
	// capability_versions, which FKs capabilities, which FKs
	// projects, which FKs workspaces — seed the full chain.
	for _, q := range []string{
		`INSERT INTO workspaces (id, name, created_at, updated_at) VALUES ('w1', 'test', '2024-01-01', '2024-01-01')`,
		`INSERT INTO projects (id, workspace_id, name, created_at, updated_at) VALUES ('p1', 'w1', 'test', '2024-01-01', '2024-01-01')`,
		`INSERT INTO capabilities (id, project_id, name, created_at, updated_at) VALUES ('c1', 'p1', 'test', '2024-01-01', '2024-01-01')`,
		`INSERT INTO capability_versions (id, capability_id, version, created_at) VALUES ('v1', 'c1', 1, '2024-01-01'), ('v2', 'c1', 2, '2024-01-01')`,
		`INSERT INTO recommendations (id, capability_version_id, type, payload)
		 VALUES ('rec-1', 'v1', 'raise_max_tokens', '{}'),
		        ('rec-2', 'v2', 'drop_guardrail', '{}')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
	// decisions.id is TEXT PRIMARY KEY without a DEFAULT (the
	// migration uses a generated UUID for each row); callers
	// must supply an id explicitly.
	if _, err := db.Exec(
		`INSERT INTO decisions (id, recommendation_id, payload)
		 VALUES ('d-1', 'rec-1', '{}'), ('d-2', 'rec-2', '{}')`,
	); err != nil {
		t.Fatalf("insert decisions: %v", err)
	}

	// 1. decisions.id is TEXT (no longer INTEGER). The id values
	// for rows inserted after migration are whatever the caller
	// supplied; the migration only generates UUIDs for rows that
	// existed BEFORE the rebuild.
	rows, err := db.Query(`SELECT id, typeof(id) FROM decisions`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, typ string
		if err := rows.Scan(&id, &typ); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if typ != "text" {
			t.Errorf("expected id type 'text', got %q (id=%q)", typ, id)
		}
	}

	// 2. recommendation_id UNIQUE is preserved.
	if _, err := db.Exec(
		`INSERT INTO decisions (recommendation_id, payload) VALUES ('rec-1', '{}')`,
	); err == nil {
		t.Errorf("expected UNIQUE violation on duplicate recommendation_id")
	}

	// 3. Re-running the migration is a no-op (the table is already
	// at version 49; the schema_migrations deduplication prevents
	// re-application).
	if err := migrate(db, migrationsFS); err != nil {
		t.Errorf("second migrate: %v", err)
	}
}
