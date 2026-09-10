import type Database from 'better-sqlite3';
import { createHash, generateKeyPairSync, createPublicKey } from 'node:crypto';
import { randomUUID } from 'node:crypto';
import type { SigningKey, SigningKeyCreateInput } from '@promptsheon/shared';

interface Row {
  id: string;
  organization_id: string;
  label: string;
  fingerprint: string;
  public_key_pem: string;
  created_by: string;
  created_at: string;
  deactivated_at: string | null;
}

function toKey(row: Row): SigningKey {
  return {
    id: row.id,
    organizationId: row.organization_id,
    label: row.label,
    fingerprint: row.fingerprint,
    publicKeyPem: row.public_key_pem,
    createdBy: row.created_by,
    createdAt: row.created_at,
    deactivatedAt: row.deactivated_at,
  };
}

/**
 * Compute the SHA-256 fingerprint of an ed25519 SPKI DER encoding.
 * The full 64-char hex is returned so the value matches what
 * openssl / ssh-keygen publish.
 */
export function fingerprintSpki(publicKeyPem: string): string {
  const keyObject = createPublicKey(publicKeyPem);
  const der = keyObject.export({ format: 'der', type: 'spki' });
  return createHash('sha256').update(der).digest('hex');
}

/** Generate an ed25519 PEM keypair (development + CI helper). */
export function generateEd25519KeyPair(): { publicKeyPem: string; privateKeyPem: string } {
  const { publicKey, privateKey } = generateKeyPairSync('ed25519');
  return {
    publicKeyPem: publicKey.export({ format: 'pem', type: 'spki' }).toString(),
    privateKeyPem: privateKey.export({ format: 'pem', type: 'pkcs8' }).toString(),
  };
}

export class SigningKeyRepo {
  constructor(private db: Database.Database) {}

  list(orgId: string): SigningKey[] {
    const rows = this.db
      .prepare(
        'SELECT * FROM signing_keys WHERE organization_id = ? AND deactivated_at IS NULL ORDER BY created_at DESC',
      )
      .all(orgId) as Row[];
    return rows.map(toKey);
  }

  findById(id: string): SigningKey | null {
    const row = this.db
      .prepare('SELECT * FROM signing_keys WHERE id = ?')
      .get(id) as Row | undefined;
    return row ? toKey(row) : null;
  }

  create(input: SigningKeyCreateInput): SigningKey {
    const fingerprint = fingerprintSpki(input.publicKeyPem);
    const id = randomUUID();
    const now = new Date().toISOString();
    this.db
      .prepare(
        `INSERT INTO signing_keys (id, organization_id, label, fingerprint, public_key_pem, created_by, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(id, input.organizationId, input.label, fingerprint, input.publicKeyPem, input.createdBy, now);
    return this.findById(id)!;
  }

  deactivate(id: string): SigningKey | null {
    this.db
      .prepare('UPDATE signing_keys SET deactivated_at = CURRENT_TIMESTAMP WHERE id = ?')
      .run(id);
    return this.findById(id);
  }
}
