import type Database from 'better-sqlite3';
import type { HealthProbe } from '../application/health-service.js';

/** SQLite implementation of the health and readiness probe boundary. */
export class SqliteHealthProbe implements HealthProbe {
  constructor(private readonly db: Database.Database) {}

  ping(): boolean {
    const result = this.db.prepare('SELECT 1 as ok').get() as { ok: number } | undefined;
    return result?.ok === 1;
  }

  quickCheck(): boolean {
    const result = this.db.prepare('PRAGMA quick_check').get() as { quick_check?: string } | undefined;
    return result?.quick_check === 'ok';
  }
}
