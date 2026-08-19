import { readdir, readFile } from 'fs/promises';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';
import type Database from 'better-sqlite3';
import { applyMigrations, type MigrationSql } from '@promptsheon/shared';

export interface MigrationRecord {
  version: number;
  name: string;
  appliedAt: string;
}

export async function runMigrations(db: Database.Database): Promise<void> {
  const currentDir = dirname(fileURLToPath(import.meta.url));
  const migrationDir = join(currentDir, '..', '..', '..', 'shared', 'db', 'migrations');

  const files = await readdir(migrationDir);
  const migrations: MigrationSql[] = [];
  for (const f of files) {
    if (!f.endsWith('.up.sql')) continue;
    const version = parseInt(f.split('_')[0], 10);
    if (Number.isNaN(version) || version === 0) continue;
    const up = await readFile(join(migrationDir, f), 'utf-8');
    migrations.push({ version, name: f, up });
  }
  applyMigrations(db, migrations);
}