import type Database from 'better-sqlite3';

export interface VaultState {
  kmsKeyId: string;
  wrappedDataKey: Buffer;
  createdAt: string;
  updatedAt: string;
}

export class VaultRepo {
  constructor(private db: Database.Database) {}

  get(): VaultState | null {
    return this.db.prepare('SELECT * FROM vault_state WHERE id = 0').get() as VaultState | null;
  }

  set(data: { kmsKeyId: string; wrappedDataKey: Buffer }): void {
    const existing = this.get();
    if (existing) {
      this.db.prepare('UPDATE vault_state SET kms_key_id = ?, wrapped_data_key = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 0')
        .run(data.kmsKeyId, data.wrappedDataKey);
    } else {
      this.db.prepare('INSERT INTO vault_state (id, kms_key_id, wrapped_data_key) VALUES (0, ?, ?)')
        .run(data.kmsKeyId, data.wrappedDataKey);
    }
  }
}
