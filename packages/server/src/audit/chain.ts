import { createHash, getFips, randomUUID } from 'node:crypto';
import type Database from 'better-sqlite3';
import type { AuditEntry } from '@promptsheon/shared';

/**
 * Compute the SHA-256 of an audit entry's hash input.
 *
 * When `fipsMode` is true the function requires Node to be running
 * against a FIPS-validated OpenSSL provider (`crypto.getFips() === 1`).
 * If FIPS isn't actually active the call throws rather than silently
 * downgrading to non-FIPS crypto — that would defeat the purpose of
 * the gate.
 *
 * Pure function: no module-level state, so it's safe to call from
 * tests and from both the append and verify paths.
 */
export function computeHash(data: string, fipsMode: boolean): string {
  if (fipsMode && getFips() !== 1) {
    throw new Error(
      'PROMPTSHEON_FIPS_MODE=true but Node is not running against a ' +
        'FIPS-validated OpenSSL provider. Refusing to compute the audit hash.',
    );
  }
  return createHash('sha256').update(data).digest('hex');
}

export class AuditChain {
  constructor(
    private db: Database.Database,
    private fipsMode = false,
  ) {
    db.prepare(
      `INSERT OR IGNORE INTO audit_chain_state (id, last_hash, last_rowid)
       VALUES (0, '', 0)`
    ).run();
  }

  /**
   * Resolve the audit_entries.user_id to a real users.id. If the
   * supplied userId doesn't exist, fall back to the system seed user
   * (created by migration 005) so background processes don't violate
   * the FK constraint.
   */
  private resolveUserId(userId: string): string {
    if (userId === 'system' || !userId) return 'api';
    const row = this.db.prepare('SELECT id FROM users WHERE id = ?').get(userId) as { id: string } | undefined;
    if (row) return userId;
    return 'api';
  }

  append(entry: {
    userId: string;
    action: string;
    resource: string;
    details: string;
    resourceKind: string;
    resourceId: string;
  }): AuditEntry {
    const chainState = this.db.prepare(
      'SELECT last_hash FROM audit_chain_state WHERE id = 0'
    ).get() as { last_hash: string } | undefined;
    const previousHash = chainState?.last_hash ?? '';

    const userId = this.resolveUserId(entry.userId);
    const hashInput = JSON.stringify({
      userId,
      action: entry.action,
      resource: entry.resource,
      details: entry.details,
      previousHash,
    });
    const entryHash = computeHash(hashInput, this.fipsMode);

    const id = randomUUID();
    const now = new Date().toISOString();

    this.db.prepare(`
      INSERT INTO audit_entries (id, user_id, action, resource, details, timestamp, previous_hash, entry_hash, timestamp_str, resource_kind, resource_id)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `).run(id, userId, entry.action, entry.resource, entry.details, now, previousHash, entryHash, now, entry.resourceKind, entry.resourceId);

    this.db.prepare(
      `INSERT INTO audit_chain_state (id, last_hash, last_rowid, updated_by_app)
       VALUES (0, ?, last_insert_rowid(), 1)
       ON CONFLICT(id) DO UPDATE SET
         last_hash = excluded.last_hash,
         last_rowid = excluded.last_rowid,
         updated_by_app = 1`
    ).run(entryHash);

    return {
      id,
      userId: entry.userId,
      action: entry.action,
      resource: entry.resource,
      details: entry.details,
      timestamp: now,
      previousHash,
      entryHash,
      timestampStr: now,
      resourceKind: entry.resourceKind,
      resourceId: entry.resourceId,
    };
  }

  verify(): { valid: boolean; brokenAt?: string } {
    const entries = this.db.prepare(
      `SELECT id,
              user_id AS userId,
              action,
              resource,
              details,
              timestamp,
              previous_hash AS previousHash,
              entry_hash AS entryHash,
              timestamp_str AS timestampStr,
              resource_kind AS resourceKind,
              resource_id AS resourceId
       FROM audit_entries
       ORDER BY rowid ASC`
    ).all() as AuditEntry[];
    let expectedPrevious = '';

    for (const entry of entries) {
      if (entry.previousHash !== expectedPrevious) {
        return { valid: false, brokenAt: entry.id };
      }
      const hashInput = JSON.stringify({
        userId: entry.userId,
        action: entry.action,
        resource: entry.resource,
        details: entry.details,
        previousHash: entry.previousHash,
      });
      // Same hash function as append() so FIPS mode is enforced
      // consistently on write and verify.
      const computedHash = computeHash(hashInput, this.fipsMode);
      if (computedHash !== entry.entryHash) {
        return { valid: false, brokenAt: entry.id };
      }
      expectedPrevious = entry.entryHash;
    }

    return { valid: true };
  }

  getChainState(): { lastHash: string; lastRowid: number } {
    return this.db.prepare(
      'SELECT last_hash as lastHash, last_rowid as lastRowid FROM audit_chain_state WHERE id = 0'
    ).get() as { lastHash: string; lastRowid: number };
  }
}
