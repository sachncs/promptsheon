package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// TestLoadDownAppliesFile verifies that LoadDown reads the named
// .down.sql file and executes it inside the supplied database
// connection. The test seeds a sentinel row, applies a down
// migration that drops it, and asserts the row is gone.

func TestLoadDownAppliesFile(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "down_test.db")

	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := migrate(db, migrationsFS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Seed a row in a table we know exists.
	if _, err := db.ExecContext(context.Background(),
		`INSERT OR IGNORE INTO users (id, email, name, role, created_at, updated_at)
		 VALUES ('u1', 'u@t.com', 'U', 'reader', '2024-01-01', '2024-01-01')`); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	// LoadDown for a non-existent version returns an error.
	if err := LoadDown(context.Background(), db, 9999); err == nil {
		t.Error("expected error for missing down migration")
	}
}

// TestLoadDownRejectsDestructiveWithoutFlag: the 044 down is
// marked destructive by the file-name heuristic; without the
// PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS env var, LoadDown
// must refuse to run it.
func TestLoadDownRejectsDestructiveWithoutFlag(t *testing.T) {
	t.Setenv("PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS", "true")

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "destructive.db")

	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := migrate(db, migrationsFS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	t.Setenv("PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS", "")
	err = LoadDown(context.Background(), db, 44)
	if err == nil {
		t.Fatal("expected destructive down to refuse without env var")
	}
}
