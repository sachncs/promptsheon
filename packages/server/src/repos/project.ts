import type { Project } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { BaseRepo, camelize, type Paginated } from './base.js';

export class ProjectRepo extends BaseRepo<Project> {
  constructor(db: Database.Database) {
    super(db, 'projects');
  }

  findByWorkspaceId(workspaceId: string): Project[] {
    return this.db.prepare('SELECT * FROM projects WHERE workspace_id = ?')
      .all(workspaceId)
      .map((row) => camelize(row as Record<string, unknown>) as unknown as Project);
  }

  findByWorkspaceIdInOrg(workspaceId: string, organizationId: string): Project[] {
    return this.db.prepare(
      `SELECT p.* FROM projects p JOIN workspaces w ON w.id = p.workspace_id
       WHERE p.workspace_id = ? AND w.org_id = ? ORDER BY p.created_at DESC`,
    ).all(workspaceId, organizationId).map((row) => camelize(row as Record<string, unknown>) as unknown as Project);
  }

  findByIdInOrg(id: string, organizationId: string): Project | null {
    const row = this.db.prepare(
      `SELECT p.* FROM projects p JOIN workspaces w ON w.id = p.workspace_id
       WHERE p.id = ? AND w.org_id = ?`,
    ).get(id, organizationId) as Record<string, unknown> | undefined;
    return row ? camelize(row) as unknown as Project : null;
  }

  findManyInOrg(organizationId: string, opts: { page: number; pageSize: number }): Paginated<Project> {
    const joins = ' FROM projects p JOIN workspaces w ON w.id = p.workspace_id WHERE w.org_id = ?';
    const total = (this.db.prepare(`SELECT COUNT(*) AS count${joins}`).get(organizationId) as { count: number }).count;
    const rows = this.db.prepare(`SELECT p.*${joins} ORDER BY p.created_at DESC LIMIT ? OFFSET ?`)
      .all(organizationId, opts.pageSize, (opts.page - 1) * opts.pageSize) as Array<Record<string, unknown>>;
    return { items: rows.map((row) => camelize(row) as unknown as Project), total };
  }

  createInOrg(data: { workspaceId: string; name: string; description?: string }, organizationId: string): Project | null {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    const result = this.db.prepare(`
      INSERT INTO projects (id, workspace_id, name, description, created_at, updated_at)
      SELECT ?, ?, ?, ?, ?, ? WHERE EXISTS (SELECT 1 FROM workspaces WHERE id = ? AND org_id = ?)
    `).run(id, data.workspaceId, data.name, data.description ?? '', now, now, data.workspaceId, organizationId);
    if (result.changes === 0) return null;
    return this.findByIdInOrg(id, organizationId);
  }

  create(data: { workspaceId: string; name: string; description?: string }): Project {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare(`INSERT INTO projects (id, workspace_id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`)
      .run(id, data.workspaceId, data.name, data.description ?? '', now, now);
    return { id, workspaceId: data.workspaceId, name: data.name, description: data.description ?? '', createdAt: now, updatedAt: now };
  }

  update(id: string, data: Partial<Pick<Project, 'name' | 'description'>>): Project | null {
    const existing = this.findById(id);
    if (!existing) return null;
    const name = data.name ?? existing.name;
    const description = data.description ?? existing.description;
    this.db.prepare(`UPDATE projects SET name = ?, description = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`)
      .run(name, description, id);
    return { ...existing, name, description };
  }

  updateInOrg(id: string, organizationId: string, data: Partial<Pick<Project, 'name' | 'description'>>): Project | null {
    const existing = this.findByIdInOrg(id, organizationId);
    if (!existing) return null;
    const name = data.name ?? existing.name;
    const description = data.description ?? existing.description;
    this.db.prepare(`UPDATE projects SET name = ?, description = ?, updated_at = CURRENT_TIMESTAMP
      WHERE id = ? AND EXISTS (SELECT 1 FROM workspaces WHERE id = projects.workspace_id AND org_id = ?)`)
      .run(name, description, id, organizationId);
    return { ...existing, name, description };
  }

  deleteInOrg(id: string, organizationId: string): boolean {
    return this.db.prepare(`DELETE FROM projects WHERE id = ? AND EXISTS
      (SELECT 1 FROM workspaces WHERE id = projects.workspace_id AND org_id = ?)`)
      .run(id, organizationId).changes > 0;
  }
}
