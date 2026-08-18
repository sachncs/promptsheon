import type Database from 'better-sqlite3';
import type { Project } from '@promptsheon/shared';
import { notFound } from '@promptsheon/shared';

export class ProjectRepo {
  constructor(private db: Database.Database) {}

  findById(id: string): Project | null {
    return this.db.prepare('SELECT * FROM projects WHERE id = ?').get(id) as Project | null;
  }

  findByWorkspaceId(workspaceId: string): Project[] {
    return this.db.prepare('SELECT * FROM projects WHERE workspace_id = ? ORDER BY created_at DESC')
      .all(workspaceId) as Project[];
  }

  findMany(opts: { page: number; pageSize: number }): { items: Project[]; total: number } {
    const total = (this.db.prepare('SELECT COUNT(*) as count FROM projects').get() as { count: number }).count;
    const items = this.db.prepare('SELECT * FROM projects LIMIT ? OFFSET ?')
      .all(opts.pageSize, (opts.page - 1) * opts.pageSize) as Project[];
    return { items, total };
  }

  create(data: { workspaceId: string; name: string; description?: string }): Project {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare('INSERT INTO projects (id, workspace_id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)')
      .run(id, data.workspaceId, data.name, data.description ?? '', now, now);
    return { id, workspaceId: data.workspaceId, name: data.name, description: data.description ?? '', createdAt: now, updatedAt: now };
  }

  update(id: string, data: Partial<Pick<Project, 'name' | 'description'>>): Project {
    const existing = this.findById(id);
    if (!existing) throw notFound('project', id);
    const now = new Date().toISOString();
    const name = data.name ?? existing.name;
    const description = data.description ?? existing.description;
    this.db.prepare('UPDATE projects SET name = ?, description = ?, updated_at = ? WHERE id = ?')
      .run(name, description, now, id);
    return { ...existing, name, description, updatedAt: now };
  }

  delete(id: string): void {
    const existing = this.findById(id);
    if (!existing) throw notFound('project', id);
    this.db.prepare('DELETE FROM projects WHERE id = ?').run(id);
  }
}
