import type { Workspace } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { BaseRepo } from './base.js';
import type { Paginated } from './base.js';

export class WorkspaceRepo extends BaseRepo<Workspace> {
  constructor(db: Database.Database) {
    super(db, 'workspaces');
  }

  create(data: { name: string; organization?: string }): Workspace {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare(`INSERT INTO workspaces (id, name, organization, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`)
      .run(id, data.name, data.organization ?? '', now, now);
    return { id, name: data.name, organization: data.organization ?? '', createdAt: now, updatedAt: now };
  }

  update(id: string, data: Partial<Pick<Workspace, 'name' | 'organization'>>): Workspace | null {
    const existing = this.findById(id);
    if (!existing) return null;
    const now = new Date().toISOString();
    const name = data.name ?? existing.name;
    const organization = data.organization ?? existing.organization;
    this.db.prepare(`UPDATE workspaces SET name = ?, organization = ?, updated_at = ? WHERE id = ?`)
      .run(name, organization, now, id);
    return { ...existing, name, organization, updatedAt: now };
  }

  findByIdInOrg(id: string, organizationId: string): Workspace | null {
    const row = this.db.prepare('SELECT * FROM workspaces WHERE id = ? AND org_id = ?').get(id, organizationId) as Record<string, unknown> | undefined;
    return row ? this.mapWorkspace(row) : null;
  }

  findManyInOrg(organizationId: string, opts: { page: number; pageSize: number }): Paginated<Workspace> {
    const total = (this.db.prepare('SELECT COUNT(*) AS count FROM workspaces WHERE org_id = ?').get(organizationId) as { count: number }).count;
    const rows = this.db.prepare('SELECT * FROM workspaces WHERE org_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?')
      .all(organizationId, opts.pageSize, (opts.page - 1) * opts.pageSize) as Array<Record<string, unknown>>;
    return { items: rows.map((row) => this.mapWorkspace(row)), total };
  }

  createInOrg(data: { name: string; organization?: string }, organizationId: string): Workspace {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare('INSERT INTO workspaces (id, name, organization, org_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)')
      .run(id, data.name, data.organization ?? '', organizationId, now, now);
    return { id, name: data.name, organization: data.organization ?? '', createdAt: now, updatedAt: now };
  }

  updateInOrg(id: string, organizationId: string, data: Partial<Pick<Workspace, 'name' | 'organization'>>): Workspace | null {
    const existing = this.findByIdInOrg(id, organizationId);
    if (!existing) return null;
    const now = new Date().toISOString();
    const name = data.name ?? existing.name;
    const organization = data.organization ?? existing.organization;
    this.db.prepare('UPDATE workspaces SET name = ?, organization = ?, updated_at = ? WHERE id = ? AND org_id = ?')
      .run(name, organization, now, id, organizationId);
    return { ...existing, name, organization, updatedAt: now };
  }

  deleteInOrg(id: string, organizationId: string): boolean {
    return this.db.prepare('DELETE FROM workspaces WHERE id = ? AND org_id = ?').run(id, organizationId).changes > 0;
  }

  private mapWorkspace(row: Record<string, unknown>): Workspace {
    return {
      id: String(row['id']),
      name: String(row['name']),
      organization: String(row['organization'] ?? ''),
      createdAt: String(row['created_at']),
      updatedAt: String(row['updated_at']),
    };
  }
}
