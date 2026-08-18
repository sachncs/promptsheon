import { readdir, readFile } from 'fs/promises';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';
import type Database from 'better-sqlite3';

export interface MigrationRecord {
  version: number;
  name: string;
  appliedAt: string;
}

export async function runMigrations(db: Database.Database): Promise<void> {
  db.exec(`
    CREATE TABLE IF NOT EXISTS _migrations (
      version INTEGER PRIMARY KEY,
      name TEXT NOT NULL,
      applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
    )
  `);

  const applied = new Set(
    db.prepare('SELECT version FROM _migrations').all().map((r: any) => r.version as number)
  );

  // Find migrations relative to this file's location
  const currentDir = dirname(fileURLToPath(import.meta.url));
  // Go up to packages/server, then into packages/shared/db/migrations
  const migrationDir = join(currentDir, '..', '..', '..', 'shared', 'db', 'migrations');

  const files = await readdir(migrationDir);
  const ups = files
    .filter(f => f.endsWith('.up.sql'))
    .map(f => ({
      version: parseInt(f.split('_')[0], 10),
      name: f,
      path: join(migrationDir, f),
    }))
    .sort((a, b) => a.version - b.version);

  for (const migration of ups) {
    if (applied.has(migration.version)) continue;
    const sql = await readFile(migration.path, 'utf-8');
    db.transaction(() => {
      db.exec(sql);
      db.prepare('INSERT INTO _migrations (version, name) VALUES (?, ?)')
        .run(migration.version, migration.name);
    })();
    console.log(`Applied migration ${migration.name}`);
  }
}
