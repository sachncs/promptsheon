import type Database from 'better-sqlite3';
import type { ProviderKey } from '@promptsheon/shared';
import { notFound } from '@promptsheon/shared';

export class ProviderKeyRepo {
  constructor(private db: Database.Database) {}

  findById(id: string): ProviderKey | null {
    return this.db.prepare('SELECT * FROM provider_keys WHERE id = ?').get(id) as ProviderKey | null;
  }

  findByName(providerName: string, keyName: string): ProviderKey | null {
    return this.db.prepare('SELECT * FROM provider_keys WHERE provider_name = ? AND key_name = ?')
      .get(providerName, keyName) as ProviderKey | null;
  }

  findByProvider(providerName: string): ProviderKey[] {
    return this.db.prepare('SELECT * FROM provider_keys WHERE provider_name = ? ORDER BY created_at DESC')
      .all(providerName) as ProviderKey[];
  }

  findMany(): ProviderKey[] {
    return this.db.prepare('SELECT * FROM provider_keys ORDER BY created_at DESC').all() as ProviderKey[];
  }

  create(data: { providerName: string; keyName: string; encryptedKey: Buffer }): ProviderKey {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare('INSERT INTO provider_keys (id, provider_name, key_name, encrypted_key, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)')
      .run(id, data.providerName, data.keyName, data.encryptedKey, now, now);
    return { id, providerName: data.providerName, keyName: data.keyName, encryptedKey: data.encryptedKey, createdAt: now, updatedAt: now, rotatedAt: null, lastUsedAt: null };
  }

  delete(id: string): void {
    const existing = this.findById(id);
    if (!existing) throw notFound('provider_key', id);
    this.db.prepare('DELETE FROM provider_keys WHERE id = ?').run(id);
  }
}
