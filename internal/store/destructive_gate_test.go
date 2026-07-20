package store

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

// TestDestructiveGate044Refusal: a fresh DB cannot complete
// migrate() when 044 is a destructive migration and the
// operator has not set PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS.
func TestDestructiveGate044Refusal(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "gate.db")

	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(ON)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	t.Setenv("PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS", "")
	err = migrate(db, migrationsFS)
	if err == nil {
		t.Fatalf("expected migrate to refuse without destructive flag")
	}
	if !strings.Contains(err.Error(), "destructive") {
		t.Errorf("expected error to mention 'destructive', got %v", err)
	}
}

// TestDestructiveGateNames: covers the heuristic on filename
// alone. The rename to 044_destructive_legacy_drop ensures the
// filename contains the substring that triggers the gate.
func TestDestructiveGateNames(t *testing.T) {
	cases := []struct {
		name     string
		filename string
		want     bool
	}{
		{"renamed legacy drop", "044_destructive_legacy_drop.up.sql", true},
		{"destructive sql", "025_destructive.sql", true},
		{"non-destructive", "001_initial.sql", false},
		{"backfill marker", "052_audit_backfill_tool_marker.up.sql", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isDestructiveMigration(c.filename); got != c.want {
				t.Errorf("isDestructiveMigration(%q) = %v, want %v", c.filename, got, c.want)
			}
		})
	}
}
