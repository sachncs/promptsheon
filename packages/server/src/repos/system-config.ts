import type Database from 'better-sqlite3';
import type { SystemConfig } from '@promptsheon/shared';

export class SystemConfigRepo {
  constructor(private db: Database.Database) {}

  get(key: string): SystemConfig | null {
    return this.db.prepare('SELECT * FROM system_config WHERE key = ?').get(key) as SystemConfig | null;
  }

  set(key: string, value: string, updatedBy?: string, tombstone = false): boolean {
    const existing = this.get(key);
    if (existing) {
      this.db.prepare(`
        UPDATE system_config
        SET value = ?, updated_at = CURRENT_TIMESTAMP, updated_by = ?,
            tombstone = ?, write_ts = write_ts + 1
        WHERE key = ?
      `).run(value, updatedBy ?? 'system', tombstone ? 1 : 0, key);
    } else {
      this.db.prepare(`
        INSERT INTO system_config (key, value, updated_by, tombstone)
        VALUES (?, ?, ?, ?)
      `).run(key, value, updatedBy ?? 'system', tombstone ? 1 : 0);
    }
    return true;
  }

  list(prefix?: string): SystemConfig[] {
    if (prefix) {
      return this.db.prepare('SELECT * FROM system_config WHERE key LIKE ?')
        .all(`${prefix}%`) as SystemConfig[];
    }
    return this.db.prepare('SELECT * FROM system_config').all() as SystemConfig[];
  }

  delete(key: string, updatedBy?: string): boolean {
    return this.set(key, 'null', updatedBy, true);
  }
}
