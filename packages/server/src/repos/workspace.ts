import type { Workspace } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { BaseRepo } from './base.js';

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
}