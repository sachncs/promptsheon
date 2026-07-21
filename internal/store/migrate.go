package store

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
)

// DestructiveMigrationEnv is the env var an operator sets to opt into
// a migration that drops tables or columns. The default is refusal:
// the daemon prints a clear error and refuses to boot rather than
// silently destroying data. The daemon philosophy is to fail
// successfully, never to hide destructive operations behind an
// upgrade.
const DestructiveMigrationEnv = "PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS"

// migrate runs all SQL migration files in order against the database.
// Migrations are expected in the migrations/ directory with names like
// "001_initial.sql", "002_add_foo.sql", etc.
//
// Safety features:
//   - Each migration runs in its own transaction (rollback on failure)
//   - Forward-only: no down migrations, migrations are idempotent
//   - Destructive migrations (file name contains "_destructive_") require
//     PROMPTSHEON_ALLOW_DESTRUCTIVE_MIGRATIONS=true to apply. Without
//     the flag, the daemon refuses to start. This is the operational
//     gate per the production-readiness review: the daemon never silently
//     drops tables or columns during a routine upgrade.
func migrate(db *sql.DB, migrationsFS fs.FS) error {
	destructiveAllowed := os.Getenv(DestructiveMigrationEnv) == "true"
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
		// Skip .down.sql by default; LoadDown loads them on
		// explicit operator request.
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		if strings.HasSuffix(e.Name(), ".down.sql") {
			continue
		}
		files = append(files, e)
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

		if isDestructiveMigration(f.Name()) && !destructiveAllowed {
			return fmt.Errorf(
				"migration %s is destructive and requires %s=true; "+
					"refusing to start. Take a backup of %s before retrying",
				f.Name(), DestructiveMigrationEnv, "<db path>",
			)
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
//
// SQLite has a quirk: PRAGMA foreign_keys cannot be changed inside a
// transaction. Migrations that need to flip the pragma off during a
// table rebuild must do it outside the surrounding transaction.
// We detect that pattern by looking for a leading or trailing
// PRAGMA foreign_keys line and running it on the connection
// before/after the transaction. The pragma is a no-op when
// foreign_keys is already in the requested state.
func applyMigration(db *sql.DB, version int, sqlStr string) error {
	pre, body := splitLeadingPragma(sqlStr)
	body, post := splitTrailingPragma(body)
	if pre != "" {
		if _, err := db.Exec(pre); err != nil {
			return fmt.Errorf("execute pre-tx pragma: %w", err)
		}
		sqlStr = body
	} else {
		sqlStr = body
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(sqlStr); err != nil {
		return fmt.Errorf("execute migration: %w", err)
	}

	if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
		return fmt.Errorf("record migration version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	if post != "" {
		if _, err := db.Exec(post); err != nil {
			return fmt.Errorf("execute post-tx pragma: %w", err)
		}
	}
	return nil
}

// splitLeadingPragma returns (prefix, body) where prefix is a
// leading "PRAGMA foreign_keys = ..." directive (if present)
// and body is the remainder. Returns ("", sqlStr) if no pragma
// leads the file.
func splitLeadingPragma(sqlStr string) (string, string) {
	trimmed := strings.TrimLeft(sqlStr, " \t\r\n")
	upper := strings.ToUpper(trimmed)
	if !strings.HasPrefix(upper, "PRAGMA FOREIGN_KEYS") {
		return "", sqlStr
	}
	end := strings.IndexAny(trimmed, ";\n")
	if end < 0 {
		return "", sqlStr
	}
	prefix := strings.TrimSpace(trimmed[:end+1])
	body := strings.TrimSpace(trimmed[end+1:])
	return prefix, body
}

// splitTrailingPragma returns (body, post) where post is a
// trailing "PRAGMA foreign_keys = ..." directive (if present)
// and body is the remainder.
func splitTrailingPragma(sqlStr string) (string, string) {
	idx := strings.LastIndex(strings.ToUpper(sqlStr), "PRAGMA FOREIGN_KEYS")
	if idx < 0 {
		return sqlStr, ""
	}
	body := strings.TrimSpace(sqlStr[:idx])
	post := strings.TrimSpace(sqlStr[idx:])
	if end := strings.IndexAny(post, ";\n"); end >= 0 {
		post = strings.TrimSpace(post[:end+1])
	}
	return body, post
}

// isDestructiveMigration reports whether a migration file is
// destructive (drops tables or columns) by its file name. The
// convention is the substring "destructive" in the file name; this
// is intentionally cheap to grep and audit. The convention is
// "_destructive_<rest>" where <rest> is anything including the file
// extension, so we match on "destructive" alone.
func isDestructiveMigration(name string) bool {
	return strings.Contains(name, "destructive")
}

// LoadDown reads and applies a single .down.sql file by version.
// The caller is the production recovery flow: an operator who
// wants to roll back a migration calls this with the version
// number (e.g. 046 to undo the alert M2M backfill).
//
// Destructive down migrations are gated on the same env var as
// forward destructive migrations; a request to undo a destructive
// migration without the env var returns an error.
func LoadDown(ctx context.Context, db *sql.DB, version int) error {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	target := fmt.Sprintf("%03d", version) + "_"
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, target) || !strings.HasSuffix(name, ".down.sql") {
			continue
		}
		if isDestructiveMigration(name) && os.Getenv(DestructiveMigrationEnv) != "true" {
			return fmt.Errorf("down %s is destructive and requires %s=true", name, DestructiveMigrationEnv)
		}
		content, err := fs.ReadFile(migrationsFS, "migrations/"+name)
		if err != nil {
			return fmt.Errorf("read down %s: %w", name, err)
		}
		if _, err := db.ExecContext(ctx, string(content)); err != nil {
			return fmt.Errorf("apply down %s: %w", name, err)
		}
		return nil
	}
	return fmt.Errorf("no down migration for version %d", version)
}
