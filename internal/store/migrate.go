package store

import (
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// migrate runs all SQL migration files in order against the database.
// Migrations are expected in the migrations/ directory with names like
// "001_initial.sql", "002_add_foo.sql", etc.
func migrate(db *sql.DB, migrationsFS fs.FS) error {
	// Ensure the schema_migrations table exists.
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	// Read applied versions.
	applied := make(map[int]bool)
	rows, err := db.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("query migrations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return fmt.Errorf("scan migration version: %w", err)
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Read migration files.
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var files []fs.DirEntry
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e)
		}
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	// Apply pending migrations.
	for _, f := range files {
		var version int
		if _, err := fmt.Sscanf(f.Name(), "%d_", &version); err != nil {
			continue
		}
		if applied[version] {
			continue
		}

		content, err := fs.ReadFile(migrationsFS, "migrations/"+f.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f.Name(), err)
		}

		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("apply migration %s: %w", f.Name(), err)
		}

		if _, err := db.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
			return fmt.Errorf("record migration %s: %w", f.Name(), err)
		}
	}

	return nil
}
