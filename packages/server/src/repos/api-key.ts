import type { ApiKey } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { BaseRepo } from './base.js';

export class ApiKeyRepo extends BaseRepo<ApiKey> {
  constructor(db: Database.Database) {
    super(db, 'api_keys');
  }

  findByKeyHash(keyHash: string): ApiKey | null {
    return this.db.prepare('SELECT * FROM api_keys WHERE key_hash = ?').get(keyHash) as ApiKey | null;
  }

  findByUserId(userId: string): ApiKey[] {
    return this.db.prepare('SELECT * FROM api_keys WHERE user_id = ? ORDER BY created_at DESC')
      .all(userId) as ApiKey[];
  }

  create(data: { userId: string; name: string; keyHash: string; keyPrefix: string; role: string; expiresAt?: string }): ApiKey {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare(`INSERT INTO api_keys (id, user_id, name, key_hash, key_prefix, role, expires_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
      .run(id, data.userId, data.name, data.keyHash, data.keyPrefix, data.role, data.expiresAt ?? null, now);
    return {
      id, userId: data.userId, name: data.name, keyHash: data.keyHash, keyPrefix: data.keyPrefix,
      role: data.role as ApiKey['role'], expiresAt: data.expiresAt ?? null, lastUsed: null,
      createdAt: now, revoked: false,
    };
  }

  updateLastUsed(id: string): void {
    this.db.prepare('UPDATE api_keys SET last_used = CURRENT_TIMESTAMP WHERE id = ?').run(id);
  }

  revoke(id: string): boolean {
    const result = this.db.prepare('UPDATE api_keys SET revoked = 1 WHERE id = ?').run(id);
    return result.changes > 0;
  }
}