import type { Project } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { BaseRepo } from './base.js';

export class ProjectRepo extends BaseRepo<Project> {
  constructor(db: Database.Database) {
    super(db, 'projects');
  }

  findByWorkspaceId(workspaceId: string): Project[] {
    return this.db.prepare('SELECT * FROM projects WHERE workspace_id = ?')
      .all(workspaceId) as Project[];
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
}