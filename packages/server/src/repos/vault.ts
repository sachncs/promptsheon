import type Database from 'better-sqlite3';
import { createCipheriv, createDecipheriv, createHash, randomBytes, randomUUID } from 'node:crypto';

export interface VaultSecret {
  id: string;
  organizationId: string;
  name: string;
  fingerprint: string;
  createdBy: string;
  createdAt: string;
  rotatedAt: string | null;
}

interface VaultRow {
  id: string;
  organization_id: string;
  name: string;
  ciphertext: string;
  fingerprint: string;
  created_by: string;
  created_at: string;
  rotated_at: string | null;
}

function toSecret(row: VaultRow): VaultSecret {
  return {
    id: row.id,
    organizationId: row.organization_id,
    name: row.name,
    fingerprint: row.fingerprint,
    createdBy: row.created_by,
    createdAt: row.created_at,
    rotatedAt: row.rotated_at,
  };
}

function fingerprint(value: string): string {
  return createHash('sha256').update(value).digest('hex').slice(0, 32);
}

function encrypt(value: string, key: Buffer): string {
  const iv = randomBytes(12);
  const cipher = createCipheriv('aes-256-gcm', key, iv);
  const enc = Buffer.concat([cipher.update(value, 'utf-8'), cipher.final()]);
  const tag = cipher.getAuthTag();
  return Buffer.concat([iv, tag, enc]).toString('base64');
}

function decrypt(blob: string, key: Buffer): string {
  const buf = Buffer.from(blob, 'base64');
  const iv = buf.subarray(0, 12);
  const tag = buf.subarray(12, 28);
  const enc = buf.subarray(28);
  const decipher = createDecipheriv('aes-256-gcm', key, iv);
  decipher.setAuthTag(tag);
  return Buffer.concat([decipher.update(enc), decipher.final()]).toString('utf-8');
}

/**
 * Vault — name-keyed secret store scoped to organisations.
 *
 * The shim layer keeps encryption functional with a stable dev
 * key derived from the server secret in `.env`. Production
 * deployments should swap for AWS Secrets Manager, HashiCorp
 * Vault, or Doppler by replacing `encrypt`/`decrypt` and
 * pointing the storage at the external service.
 */
export class VaultRepo {
  private devKey: Buffer = createHash('sha256').update('promptsheon-dev-vault-key').digest();

  constructor(private db: Database.Database) {}

  list(orgId: string): VaultSecret[] {
    const rows = this.db
      .prepare('SELECT * FROM vault_secrets WHERE organization_id = ? ORDER BY name ASC')
      .all(orgId) as VaultRow[];
    return rows.map(toSecret);
  }

  set(orgId: string, name: string, value: string, createdBy: string): VaultSecret {
    const fp = fingerprint(value);
    const existing = this.db
      .prepare('SELECT id FROM vault_secrets WHERE organization_id = ? AND name = ?')
      .get(orgId, name) as { id: string } | undefined;
    const id = existing?.id ?? randomUUID();
    const cipher = encrypt(value, this.devKey);
    if (existing) {
      this.db
        .prepare(
          `UPDATE vault_secrets SET ciphertext = ?, fingerprint = ?,
             rotated_at = CURRENT_TIMESTAMP WHERE id = ?`,
        )
        .run(cipher, fp, id);
    } else {
      this.db
        .prepare(
          `INSERT INTO vault_secrets (id, organization_id, name, ciphertext, fingerprint, created_by)
           VALUES (?, ?, ?, ?, ?, ?)`,
        )
        .run(id, orgId, name, cipher, fp, createdBy);
    }
    return this.list(orgId).find((v) => v.id === id)!;
  }

  resolve(orgId: string, name: string): string | null {
    const row = this.db
      .prepare('SELECT ciphertext FROM vault_secrets WHERE organization_id = ? AND name = ?')
      .get(orgId, name) as { ciphertext: string } | undefined;
    if (!row) return null;
    return decrypt(row.ciphertext, this.devKey);
  }

  resolveReference(orgId: string, ref: string): string | null {
    const m = /^vault:\/\/([^/]+)\/(.+)$/.exec(ref);
    if (!m) return null;
    const [, , name] = m;
    return this.resolve(orgId, name);
  }
}
