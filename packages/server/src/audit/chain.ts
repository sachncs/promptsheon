import { createHash } from 'node:crypto';
import type Database from 'better-sqlite3';
import type { AuditEntry } from '@promptsheon/shared';

export class AuditChain {
  constructor(private db: Database.Database) {}

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

    const hashInput = JSON.stringify({
      userId: entry.userId,
      action: entry.action,
      resource: entry.resource,
      details: entry.details,
      previousHash,
    });
    const entryHash = createHash('sha256').update(hashInput).digest('hex');

    const id = crypto.randomUUID();
    const now = new Date().toISOString();

    this.db.prepare(`
      INSERT INTO audit_entries (id, user_id, action, resource, details, timestamp, previous_hash, entry_hash, timestamp_str, resource_kind, resource_id)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `).run(id, entry.userId, entry.action, entry.resource, entry.details, now, previousHash, entryHash, now, entry.resourceKind, entry.resourceId);

    this.db.prepare(
      'UPDATE audit_chain_state SET last_hash = ?, last_rowid = (SELECT last_insert_rowid()) WHERE id = 0'
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
      'SELECT * FROM audit_entries ORDER BY rowid ASC'
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
      const computedHash = createHash('sha256').update(hashInput).digest('hex');
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
