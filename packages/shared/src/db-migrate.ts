import type Database from 'better-sqlite3';

export interface MigrationRecord {
  version: number;
  name: string;
  appliedAt: string;
}

export interface MigrationSql {
  version: number;
  name: string;
  up: string;
}

/**
 * Apply a batch of SQL migrations to the database in version order.
 * Each migration is applied in a transaction with its _migrations row.
 * Already-applied migrations (by version) are skipped.
 *
 * @param db better-sqlite3 database connection
 * @param migrations list of migration SQL with version numbers
 */
export function applyMigrations(
  db: Database.Database,
  migrations: MigrationSql[],
): void {
  db.exec(`
    CREATE TABLE IF NOT EXISTS _migrations (
      version INTEGER PRIMARY KEY,
      name TEXT NOT NULL,
      applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
    )
  `);

  const applied = new Set<number>(
    (db.prepare('SELECT version FROM _migrations').all() as Array<{ version: number }>).map(
      (r) => r.version,
    ),
  );

  const sorted = [...migrations].sort((a, b) => a.version - b.version);

  for (const migration of sorted) {
    if (applied.has(migration.version)) continue;
    db.transaction(() => {
      db.exec(migration.up);
      db.prepare('INSERT INTO _migrations (version, name) VALUES (?, ?)')
        .run(migration.version, migration.name);
    })();
  }
}

export function appliedMigrations(db: Database.Database): MigrationRecord[] {
  const rows = db.prepare('SELECT version, name, applied_at FROM _migrations ORDER BY version')
    .all() as Array<{ version: number; name: string; applied_at: string }>;
  return rows.map((r) => ({
    version: r.version,
    name: r.name,
    appliedAt: r.applied_at,
  }));
}