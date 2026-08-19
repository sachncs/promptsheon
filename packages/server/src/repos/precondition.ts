import type { Precondition } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { BaseRepo } from './base.js';

export class PreconditionRepo extends BaseRepo<Precondition> {
  constructor(db: Database.Database) {
    super(db, 'preconditions');
  }

  findByCapabilityId(capabilityId: string): Precondition[] {
    return this.db.prepare('SELECT * FROM preconditions WHERE capability_id = ?')
      .all(capabilityId) as Precondition[];
  }

  create(data: { capabilityId: string; name: string; command: string; timeoutSec?: number; enabled?: boolean }): Precondition {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare(`INSERT INTO preconditions (id, capability_id, name, command, timeout_sec, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
      .run(id, data.capabilityId, data.name, data.command, data.timeoutSec ?? 30, data.enabled ? 1 : 0, now, now);
    return {
      id, capabilityId: data.capabilityId, name: data.name, command: data.command,
      timeoutSec: data.timeoutSec ?? 30, enabled: data.enabled ?? true,
      createdAt: now,
    };
  }
}