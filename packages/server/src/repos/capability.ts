import type Database from 'better-sqlite3';
import { NotFoundError } from '@promptsheon/shared';
import type { Capability } from '@promptsheon/shared';

export class CapabilityRepo {
  constructor(private db: Database.Database) {}

  findById(id: string): Capability | null {
    return this.db.prepare('SELECT * FROM capabilities WHERE id = ?').get(id) as Capability | null;
  }

  findByProjectId(projectId: string): Capability[] {
    return this.db.prepare('SELECT * FROM capabilities WHERE project_id = ? ORDER BY created_at DESC')
      .all(projectId) as Capability[];
  }

  findMany(opts: { page: number; pageSize: number }): { items: Capability[]; total: number } {
    const total = (this.db.prepare('SELECT COUNT(*) as count FROM capabilities').get() as { count: number }).count;
    const items = this.db.prepare('SELECT * FROM capabilities LIMIT ? OFFSET ?')
      .all(opts.pageSize, (opts.page - 1) * opts.pageSize) as Capability[];
    return { items, total };
  }

  create(data: { projectId: string; name: string; description?: string }): Capability {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare(`
      INSERT INTO capabilities (id, project_id, name, description, created_at, updated_at,
        self_evolve_enabled, self_evolve_min_score, self_evolve_max_revisions,
        self_evolve_cooldown_sec, self_evolve_target_env, self_evolve_dataset_id)
      VALUES (?, ?, ?, ?, ?, ?, 0, 0.9, 10, 900, 'dev', '')
    `).run(id, data.projectId, data.name, data.description ?? '', now, now);
    return {
      id, projectId: data.projectId, name: data.name, description: data.description ?? '',
      createdAt: now, updatedAt: now, selfEvolveEnabled: false, selfEvolveMinScore: 0.9,
      selfEvolveMaxRevisions: 10, selfEvolveCooldownSec: 900, selfEvolveTargetEnv: 'dev',
      selfEvolveDatasetId: '',
    };
  }

  update(id: string, data: Partial<Omit<Capability, 'id' | 'createdAt' | 'updatedAt'>>): Capability | null {
    const existing = this.findById(id);
    if (!existing) throw new NotFoundError("resource", id);
    const now = new Date().toISOString();
    const fields: string[] = [];
    const values: unknown[] = [];
    for (const [key, value] of Object.entries(data)) {
      if (value === undefined) continue;
      const col = key.replace(/([A-Z])/g, '_$1').toLowerCase();
      fields.push(`${col} = ?`);
      values.push(value);
    }
    if (fields.length > 0) {
      fields.push('updated_at = ?');
      values.push(now);
      values.push(id);
      this.db.prepare(`UPDATE capabilities SET ${fields.join(', ')} WHERE id = ?`).run(...values);
    }
    return { ...existing, ...data, updatedAt: now };
  }

  delete(id: string): boolean {
    const existing = this.findById(id);
    if (!existing) throw new NotFoundError("resource", id);
    this.db.prepare('DELETE FROM capabilities WHERE id = ?').run(id);
    return true;
  }
}
