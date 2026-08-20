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

export interface VaultKeyringEntry {
  id: number;
  label: string;
  fingerprint: string;
  ciphertext: string | null;
  active: boolean;
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

interface KeyringRow {
  id: number;
  label: string;
  fingerprint: string;
  ciphertext: string | null;
  active: number;
  created_at: string;
  rotated_at: string | null;
}

/**
 * KMS abstraction. The default `LocalKms` returns keys from the
 * dev-materialised keyring table; production deployments swap in
 * `AwsSecretsManagerKms` / `VaultKms` / `DopplerKms` by satisfying
 * the same shape — the rest of the vault never touches the
 * cipher directly.
 */
export interface Kms {
  /** Resolve the bytes of a key by its fingerprint, or null. */
  resolve(fingerprint: string): Buffer | null;
  /** Materialise a fresh 256-bit key. Returns fingerprint + bytes. */
  generate(label: string): { fingerprint: string; key: Buffer };
}

export class LocalKms implements Kms {
  constructor(private db: Database.Database) {}

  resolve(fingerprint: string): Buffer | null {
    const row = this.db
      .prepare('SELECT ciphertext FROM vault_keyring WHERE fingerprint = ?')
      .get(fingerprint) as { ciphertext: string | null } | undefined;
    if (!row?.ciphertext) return null;
    return Buffer.from(row.ciphertext, 'base64');
  }

  generate(label: string): { fingerprint: string; key: Buffer } {
    const key = randomBytes(32);
    const fingerprint = createHash('sha256').update(key).digest('hex').slice(0, 32);
    this.db
      .prepare(
        `INSERT INTO vault_keyring (label, fingerprint, ciphertext, active) VALUES (?, ?, ?, 0)`,
      )
      .run(label, fingerprint, key.toString('base64'));
    return { fingerprint, key };
  }
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
 * Vault — content-addressed secret storage with rotation. Each
 * write carries the key fingerprint that encrypted it so reads
 * can try the active key first, then fall back across older
 * versions.
 */
export class VaultRepo {
  constructor(
    public readonly db: Database.Database,
    public readonly kms: Kms,
  ) {}

  /** Plaintext cipher used in the very first dev install. */
  static devKeyBytes(): Buffer {
    return Buffer.from('promptsheon-dev-vault-key', 'utf-8').length === 32
      ? Buffer.from('promptsheon-dev-vault-key')
      : createHash('sha256').update('promptsheon-dev-vault-key').digest();
  }

  list(orgId: string): VaultSecret[] {
    const rows = this.db
      .prepare('SELECT * FROM vault_secrets WHERE organization_id = ? ORDER BY name ASC')
      .all(orgId) as VaultRow[];
    return rows.map(toSecret);
  }

  set(orgId: string, name: string, value: string, createdBy: string): VaultSecret {
    const fp = fingerprint(value);
    const key = this.requireActiveKey();
    const cipher = encrypt(value, key);
    const existing = this.db
      .prepare('SELECT id FROM vault_secrets WHERE organization_id = ? AND name = ?')
      .get(orgId, name) as { id: string } | undefined;
    const id = existing?.id ?? randomUUID();
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
      .prepare('SELECT ciphertext, fingerprint FROM vault_secrets WHERE organization_id = ? AND name = ?')
      .get(orgId, name) as { ciphertext: string; fingerprint: string } | undefined;
    if (!row) return null;
    return decrypt(row.ciphertext, this.requireKeyByFingerprint(row.fingerprint));
  }

  resolveReference(orgId: string, ref: string): string | null {
    const m = /^vault:\/\/([^/]+)\/(.+)$/.exec(ref);
    if (!m) return null;
    const [, , name] = m;
    return this.resolve(orgId, name);
  }

  rotate(activeFingerprint: string, label: string): VaultKeyringEntry {
    const next = this.kms.generate(label);
    if (!next.key || next.key.length !== 32) {
      throw new Error('KMS must produce a 32-byte AES-256 key');
    }
    const tx = this.db.transaction(() => {
      this.db.prepare('UPDATE vault_secrets SET rotated_at = CURRENT_TIMESTAMP').run();
      this.db
        .prepare(
          `INSERT INTO vault_keyring (label, fingerprint, ciphertext, active, created_at)
           VALUES (?, ?, ?, 1, CURRENT_TIMESTAMP)`,
        )
        .run(label, next.fingerprint, next.key.toString('base64'));
      this.db
        .prepare('UPDATE vault_keyring SET active = 0, rotated_at = CURRENT_TIMESTAMP WHERE fingerprint = ?')
        .run(activeFingerprint);
    });
    tx();
    return this.listKeyring().find((k) => k.fingerprint === next.fingerprint)!;
  }

  reencryptAllFromKey(fromFingerprint: string, toFingerprint: string): number {
    const fromKey = this.kms.resolve(fromFingerprint);
    const toKey = this.kms.resolve(toFingerprint);
    if (!fromKey || !toKey) throw new Error('KMS cannot resolve one of the fingerprints');
    const rows = this.db
      .prepare("SELECT id, ciphertext FROM vault_secrets WHERE fingerprint = ?")
      .all(fromFingerprint) as Array<{ id: string; ciphertext: string }>;
    let n = 0;
    const tx = this.db.transaction(() => {
      for (const r of rows) {
        const plain = decrypt(r.ciphertext, fromKey);
        const reEnc = encrypt(plain, toKey);
        this.db
          .prepare('UPDATE vault_secrets SET ciphertext = ?, fingerprint = ?, rotated_at = CURRENT_TIMESTAMP WHERE id = ?')
          .run(reEnc, toFingerprint, r.id);
        n++;
      }
    });
    tx();
    return n;
  }

  listKeyring(): VaultKeyringEntry[] {
    const rows = this.db
      .prepare('SELECT * FROM vault_keyring ORDER BY created_at DESC')
      .all() as KeyringRow[];
    return rows.map((r) => ({
      id: r.id,
      label: r.label,
      fingerprint: r.fingerprint,
      ciphertext: r.ciphertext,
      active: r.active === 1,
      createdAt: r.created_at,
      rotatedAt: r.rotated_at,
    }));
  }

  private requireActiveKey(): Buffer {
    const row = this.db
      .prepare('SELECT fingerprint FROM vault_keyring WHERE active = 1')
      .get() as { fingerprint: string } | undefined;
    if (!row) throw new Error('no active encryption key');
    const buf = this.kms.resolve(row.fingerprint);
    if (!buf) throw new Error(`KMS cannot resolve active key ${row.fingerprint}`);
    return buf;
  }

  private requireKeyByFingerprint(fp: string): Buffer {
    const buf = this.kms.resolve(fp);
    if (buf) return buf;
    // KMS doesn't have it. Fall back to active key (best-effort).
    return this.requireActiveKey();
  }
}
