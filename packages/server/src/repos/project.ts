import type { Project } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { z } from 'zod';
import { BaseRepo, type Paginated } from './base.js';

const ProjectRowSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  name: z.string(),
  description: z.string(),
  created_at: z.string(),
  updated_at: z.string(),
});

const CountSchema = z.object({ count: z.number().int().nonnegative() });

function toProject(row: unknown): Project {
  const value = ProjectRowSchema.parse(row);
  return {
    id: value.id,
    workspaceId: value.workspace_id,
    name: value.name,
    description: value.description,
    createdAt: value.created_at,
    updatedAt: value.updated_at,
  };
}

export class ProjectRepo extends BaseRepo<Project> {
  constructor(db: Database.Database) {
    super(db, 'projects');
  }

  override findById(id: string): Project | null {
    const row = this.db.prepare('SELECT * FROM projects WHERE id = ?').get(id);
    return row ? toProject(row) : null;
  }

  override findMany(opts: { page: number; pageSize: number }): Paginated<Project> {
    const total = CountSchema.parse(this.db.prepare('SELECT COUNT(*) AS count FROM projects').get()).count;
    const rows = this.db.prepare('SELECT * FROM projects LIMIT ? OFFSET ?')
      .all(opts.pageSize, (opts.page - 1) * opts.pageSize);
    return { items: rows.map(toProject), total };
  }

  findByWorkspaceId(workspaceId: string): Project[] {
    return this.db.prepare('SELECT * FROM projects WHERE workspace_id = ?')
      .all(workspaceId)
      .map(toProject);
  }

  findByWorkspaceIdInOrg(workspaceId: string, organizationId: string): Project[] {
    return this.db.prepare(
      `SELECT p.* FROM projects p JOIN workspaces w ON w.id = p.workspace_id
       WHERE p.workspace_id = ? AND w.org_id = ? ORDER BY p.created_at DESC`,
    ).all(workspaceId, organizationId).map(toProject);
  }

  findByIdInOrg(id: string, organizationId: string): Project | null {
    const row = this.db.prepare(
      `SELECT p.* FROM projects p JOIN workspaces w ON w.id = p.workspace_id
       WHERE p.id = ? AND w.org_id = ?`,
    ).get(id, organizationId);
    return row ? toProject(row) : null;
  }

  findManyInOrg(organizationId: string, opts: { page: number; pageSize: number }): Paginated<Project> {
    const joins = ' FROM projects p JOIN workspaces w ON w.id = p.workspace_id WHERE w.org_id = ?';
    const total = CountSchema.parse(this.db.prepare(`SELECT COUNT(*) AS count${joins}`).get(organizationId)).count;
    const rows = this.db.prepare(`SELECT p.*${joins} ORDER BY p.created_at DESC LIMIT ? OFFSET ?`)
      .all(organizationId, opts.pageSize, (opts.page - 1) * opts.pageSize);
    return { items: rows.map(toProject), total };
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
