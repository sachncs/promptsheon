import type Database from 'better-sqlite3';

export interface IdempotencyRecord {
  key: string;
  expiresAt: string;
  statusCode: number;
  headers: string;
  body: Buffer;
}

export class IdempotencyRepo {
  constructor(private db: Database.Database) {}

  get(key: string): IdempotencyRecord | null {
    return this.db.prepare('SELECT * FROM idempotency_cache WHERE key = ? AND expires_at > CURRENT_TIMESTAMP')
      .get(key) as IdempotencyRecord | null;
  }

  set(key: string, statusCode: number, headers: string, body: Buffer, expiresAt: string): boolean {
    this.db.prepare('INSERT OR REPLACE INTO idempotency_cache (key, expires_at, status_code, headers, body) VALUES (?, ?, ?, ?, ?)')
      .run(key, expiresAt, statusCode, headers, body);
    return true;
  }

  cleanup(): number {
    const result = this.db.prepare('DELETE FROM idempotency_cache WHERE expires_at <= CURRENT_TIMESTAMP').run();
    return result.changes;
  }
}
