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
//
// Safety features:
//   - Each migration runs in its own transaction (rollback on failure)
//   - Forward-only: no down migrations, migrations are idempotent
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
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var v int
		if e := rows.Scan(&v); e != nil {
			return fmt.Errorf("scan migration version: %w", e)
		}
		applied[v] = true
	}
	if e := rows.Err(); e != nil {
		return e
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

		// Apply migration in a transaction.
		if err := applyMigration(db, version, string(content)); err != nil {
			return fmt.Errorf("apply migration %s: %w", f.Name(), err)
		}
	}

	return nil
}

// applyMigration applies a single migration within a transaction.
// If the migration fails, the transaction is rolled back.
func applyMigration(db *sql.DB, version int, sqlStr string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Execute the migration SQL.
	if _, err := tx.Exec(sqlStr); err != nil {
		return fmt.Errorf("execute migration: %w", err)
	}

	// Record the migration version.
	if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
		return fmt.Errorf("record migration version: %w", err)
	}

	// Commit the transaction.
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
