//go:build tests_migration


package store

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestNewSQLite_PrepareFailureWrapped verifies the Phase 1.8
// fix: when a prepared statement cannot be built (because the
// migration did not create the expected schema), NewSQLite
// returns a wrapped error rather than silently leaving the
// statement nil and falling back to per-call SQL.
//
// We trigger the failure by opening a fresh database, deleting
// the releases table, then calling NewSQLite on a path that has
// already had its migrations applied but whose schema no longer
// matches the prepared query.
func TestNewSQLite_PrepareFailureWrapped(t *testing.T) {
	t.Setenv("PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS", "true")
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "prepare.db")

	// First, open and apply the full migration set so the
	// schema is established.
	s, err := NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("initial NewSQLite: %v", err)
	}
	if _, err := s.db.Exec(`DROP TABLE IF EXISTS releases`); err != nil {
		t.Fatalf("drop releases: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Re-open the same file. Migration is a no-op (no
	// migrations left to apply), so the releases table is
	// still missing. The hot-path statement prepare must
	// fail, and the error must wrap a "prepare statement:"
	// prefix so operators can identify the cause.
	_, err = NewSQLite(dbPath)
	if err == nil {
		t.Fatal("expected NewSQLite to fail when prepared statement cannot be built")
	}
	if !strings.Contains(err.Error(), "prepare statement") {
		t.Errorf("expected wrapped prepare error, got %v", err)
	}
}
