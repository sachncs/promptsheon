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
      createdAt: now, updatedAt: now,
    };
  }

  update(id: string, data: { name?: string; command?: string; timeoutSec?: number; enabled?: boolean }): Precondition | null {
    const row = this.db.prepare(
      'SELECT id, capability_id, name, command, timeout_sec, enabled, created_at, updated_at FROM preconditions WHERE id = ?',
    ).get(id) as
      | {
          id: string;
          capability_id: string;
          name: string;
          command: string;
          timeout_sec: number;
          enabled: number;
          created_at: string;
          updated_at: string;
        }
      | undefined;
    if (!row) return null;
    const now = new Date().toISOString();
    const next = {
      id: row.id,
      capabilityId: row.capability_id,
      name: data.name ?? row.name,
      command: data.command ?? row.command,
      timeoutSec: data.timeoutSec ?? row.timeout_sec,
      enabled: (data.enabled ?? row.enabled === 1),
      createdAt: row.created_at,
    };
    this.db.prepare(
      `UPDATE preconditions SET name = ?, command = ?, timeout_sec = ?, enabled = ?, updated_at = ?
       WHERE id = ?`,
    ).run(next.name, next.command, next.timeoutSec, next.enabled ? 1 : 0, now, id);
    return { ...next, updatedAt: now };
  }
}