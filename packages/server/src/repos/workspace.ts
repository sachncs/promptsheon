import type Database from 'better-sqlite3';
import { NotFoundError } from '@promptsheon/shared';
import type { Workspace } from '@promptsheon/shared';

export class WorkspaceRepo {
  constructor(private db: Database.Database) {}

  findById(id: string): Workspace | null {
    return this.db.prepare('SELECT * FROM workspaces WHERE id = ?').get(id) as Workspace | null;
  }

  findMany(opts: { page: number; pageSize: number }): { items: Workspace[]; total: number } {
    const total = (this.db.prepare('SELECT COUNT(*) as count FROM workspaces').get() as { count: number }).count;
    const items = this.db.prepare('SELECT * FROM workspaces LIMIT ? OFFSET ?')
      .all(opts.pageSize, (opts.page - 1) * opts.pageSize) as Workspace[];
    return { items, total };
  }

  create(data: { name: string; organization?: string }): Workspace {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare('INSERT INTO workspaces (id, name, organization, created_at, updated_at) VALUES (?, ?, ?, ?, ?)')
      .run(id, data.name, data.organization ?? '', now, now);
    return { id, name: data.name, organization: data.organization ?? '', createdAt: now, updatedAt: now };
  }

  update(id: string, data: Partial<Pick<Workspace, 'name' | 'organization'>>): Workspace | null {
    const existing = this.findById(id);
    if (!existing) throw new NotFoundError("resource", id);
    const now = new Date().toISOString();
    const name = data.name ?? existing.name;
    const organization = data.organization ?? existing.organization;
    this.db.prepare('UPDATE workspaces SET name = ?, organization = ?, updated_at = ? WHERE id = ?')
      .run(name, organization, now, id);
    return { ...existing, name, organization, updatedAt: now };
  }

  delete(id: string): boolean {
    const existing = this.findById(id);
    if (!existing) throw new NotFoundError("resource", id);
    this.db.prepare('DELETE FROM workspaces WHERE id = ?').run(id);
    return true;
  }
}
