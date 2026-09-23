import type { ApiKey } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { BaseRepo } from './base.js';

function toApiKey(row: Record<string, unknown>): ApiKey {
  const requiredString = (key: string): string => {
    const value = row[key];
    if (typeof value !== 'string') throw new Error(`api key row is missing ${key}`);
    return value;
  };
  const nullableString = (key: string): string | null => {
    const value = row[key];
    return typeof value === 'string' ? value : null;
  };
  return {
    id: requiredString('id'),
    userId: requiredString('user_id'),
    name: requiredString('name'),
    keyHash: requiredString('key_hash'),
    keyPrefix: requiredString('key_prefix'),
    role: requiredString('role') as ApiKey['role'],
    expiresAt: nullableString('expires_at'),
    lastUsed: nullableString('last_used'),
    createdAt: requiredString('created_at'),
    revoked: Boolean(row.revoked),
  };
}

export class ApiKeyRepo extends BaseRepo<ApiKey> {
  constructor(db: Database.Database) {
    super(db, 'api_keys');
  }

  findByKeyHash(keyHash: string): ApiKey | null {
    const row = this.db.prepare('SELECT * FROM api_keys WHERE key_hash = ?').get(keyHash) as Record<string, unknown> | undefined;
    return row ? toApiKey(row) : null;
  }

  findByUserId(userId: string): ApiKey[] {
    return this.db.prepare('SELECT * FROM api_keys WHERE user_id = ? ORDER BY created_at DESC')
      .all(userId)
      .map((row) => toApiKey(row as Record<string, unknown>));
  }

  listForOrg(organizationId: string): ApiKey[] {
    return this.db
      .prepare(
        `SELECT k.* FROM api_keys k
         JOIN org_members m ON m.user_id = k.user_id
         WHERE m.org_id = ? ORDER BY k.created_at ASC`,
      )
      .all(organizationId)
      .map((row) => toApiKey(row as Record<string, unknown>));
  }

  userBelongsToOrg(userId: string, organizationId: string): boolean {
    const row = this.db
      .prepare('SELECT 1 AS present FROM org_members WHERE user_id = ? AND org_id = ?')
      .get(userId, organizationId) as { present: number } | undefined;
    return row !== undefined;
  }

  revokeInOrg(id: string, organizationId: string): boolean {
    const result = this.db
      .prepare(
        `UPDATE api_keys SET revoked = 1
         WHERE id = ? AND user_id IN (SELECT user_id FROM org_members WHERE org_id = ?)`,
      )
      .run(id, organizationId);
    return result.changes > 0;
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
