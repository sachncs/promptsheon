package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestDestructiveGateRefusesWithoutFlag runs serially because it
// mutates process-level environment state shared with other tests
// in this package.
func TestDestructiveGateRefusesWithoutFlag(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "fresh.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	prev, had := os.LookupEnv(DestructiveMigrationEnv)
	os.Unsetenv(DestructiveMigrationEnv)
	t.Cleanup(func() {
		if had {
			os.Setenv(DestructiveMigrationEnv, prev)
		}
	})

	err = migrate(db, migrationsFS)
	if err == nil {
		t.Fatal("expected refusal without env var, got nil")
	}
	t.Logf("got refusal: %v", err)
}

// TestDestructiveGatePassesWithFlag runs serially because it
// mutates process-level environment state shared with other tests
// in this package.
func TestDestructiveGatePassesWithFlag(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "withflag.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	prev := os.Getenv(DestructiveMigrationEnv)
	os.Setenv(DestructiveMigrationEnv, "true")
	t.Cleanup(func() { os.Setenv(DestructiveMigrationEnv, prev) })

	if err := migrate(db, migrationsFS); err != nil {
		t.Fatalf("expected success with flag, got: %v", err)
	}
}
