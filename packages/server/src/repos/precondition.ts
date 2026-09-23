import type { Precondition } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { BaseRepo, camelize } from './base.js';

export class PreconditionRepo extends BaseRepo<Precondition> {
  constructor(db: Database.Database) {
    super(db, 'preconditions');
  }

  findByCapabilityId(capabilityId: string): Precondition[] {
    return this.db.prepare('SELECT * FROM preconditions WHERE capability_id = ?')
      .all(capabilityId) as Precondition[];
  }

  findByCapabilityIdInOrg(capabilityId: string, organizationId: string): Precondition[] {
    return this.db.prepare(
      `SELECT pc.* FROM preconditions pc
       JOIN capabilities c ON c.id = pc.capability_id
       JOIN projects p ON p.id = c.project_id
       JOIN workspaces w ON w.id = p.workspace_id
       WHERE pc.capability_id = ? AND w.org_id = ? ORDER BY pc.created_at ASC`,
    ).all(capabilityId, organizationId).map((row) => {
      const value = camelize(row as Record<string, unknown>) as unknown as Precondition;
      return { ...value, enabled: Boolean(value.enabled) };
    });
  }

  findByIdInOrg(id: string, organizationId: string): Precondition | null {
    const row = this.db.prepare(
      `SELECT pc.* FROM preconditions pc
       JOIN capabilities c ON c.id = pc.capability_id
       JOIN projects p ON p.id = c.project_id
       JOIN workspaces w ON w.id = p.workspace_id
       WHERE pc.id = ? AND w.org_id = ?`,
    ).get(id, organizationId) as Record<string, unknown> | undefined;
    if (!row) return null;
    const value = camelize(row) as unknown as Precondition;
    return { ...value, enabled: Boolean(value.enabled) };
  }

  createInOrg(data: { capabilityId: string; name: string; command: string; timeoutSec?: number; enabled?: boolean }, organizationId: string): Precondition | null {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    const result = this.db.prepare(`
      INSERT INTO preconditions (id, capability_id, name, command, timeout_sec, enabled, created_at, updated_at)
      SELECT ?, ?, ?, ?, ?, ?, ?, ?
      WHERE EXISTS (
        SELECT 1 FROM capabilities c JOIN projects p ON p.id = c.project_id
        JOIN workspaces w ON w.id = p.workspace_id
        WHERE c.id = ? AND w.org_id = ?
      )
    `).run(id, data.capabilityId, data.name, data.command, data.timeoutSec ?? 30, data.enabled === false ? 0 : 1, now, now, data.capabilityId, organizationId);
    if (result.changes === 0) return null;
    return this.findByIdInOrg(id, organizationId);
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

  updateInOrg(id: string, organizationId: string, data: { name?: string; command?: string; timeoutSec?: number; enabled?: boolean }): Precondition | null {
    const existing = this.findByIdInOrg(id, organizationId);
    if (!existing) return null;
    const now = new Date().toISOString();
    const next = {
      ...existing,
      name: data.name ?? existing.name,
      command: data.command ?? existing.command,
      timeoutSec: data.timeoutSec ?? existing.timeoutSec,
      enabled: data.enabled ?? existing.enabled,
      updatedAt: now,
    };
    this.db.prepare(
      `UPDATE preconditions SET name = ?, command = ?, timeout_sec = ?, enabled = ?, updated_at = ?
       WHERE id = ? AND EXISTS (
         SELECT 1 FROM capabilities c JOIN projects p ON p.id = c.project_id
         JOIN workspaces w ON w.id = p.workspace_id
         WHERE c.id = preconditions.capability_id AND w.org_id = ?
       )`,
    ).run(next.name, next.command, next.timeoutSec, next.enabled ? 1 : 0, now, id, organizationId);
    return next;
  }

  deleteInOrg(id: string, organizationId: string): boolean {
    const result = this.db.prepare(
      `DELETE FROM preconditions WHERE id = ? AND EXISTS (
        SELECT 1 FROM capabilities c JOIN projects p ON p.id = c.project_id
        JOIN workspaces w ON w.id = p.workspace_id
        WHERE c.id = preconditions.capability_id AND w.org_id = ?
      )`,
    ).run(id, organizationId);
    return result.changes > 0;
  }
}
