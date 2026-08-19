import type Database from 'better-sqlite3';
import { NotFoundError } from '@promptsheon/shared';
import type { Precondition } from '@promptsheon/shared';

export class PreconditionRepo {
  constructor(private db: Database.Database) {}

  findById(id: string): Precondition | null {
    return this.db.prepare('SELECT * FROM preconditions WHERE id = ?').get(id) as Precondition | null;
  }

  findByCapabilityId(capabilityId: string): Precondition[] {
    return this.db.prepare('SELECT * FROM preconditions WHERE capability_id = ? ORDER BY created_at')
      .all(capabilityId) as Precondition[];
  }

  create(data: { capabilityId: string; name: string; command: string; timeoutSec?: number; enabled?: boolean }): Precondition {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare('INSERT INTO preconditions (id, capability_id, name, command, timeout_sec, enabled, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)')
      .run(id, data.capabilityId, data.name, data.command, data.timeoutSec ?? 60, data.enabled !== false ? 1 : 0, now);
    return { id, capabilityId: data.capabilityId, name: data.name, command: data.command, timeoutSec: data.timeoutSec ?? 60, enabled: data.enabled !== false, createdAt: now };
  }

  delete(id: string): boolean {
    const existing = this.findById(id);
    if (!existing) throw new NotFoundError("resource", id);
    this.db.prepare('DELETE FROM preconditions WHERE id = ?').run(id);
    return true;
  }
}
