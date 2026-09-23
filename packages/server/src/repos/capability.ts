import type { Capability } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { BaseRepo, camelize } from './base.js';

function toCapability(row: Record<string, unknown>): Capability {
  const value = camelize(row) as unknown as Capability;
  return { ...value, selfEvolveEnabled: Boolean(row.self_evolve_enabled) };
}

export class CapabilityRepo extends BaseRepo<Capability> {
  constructor(db: Database.Database) {
    super(db, 'capabilities');
  }

  override findById(id: string): Capability | null {
    const row = this.db.prepare('SELECT * FROM capabilities WHERE id = ?').get(id) as Record<string, unknown> | undefined;
    return row ? toCapability(row) : null;
  }

  findByProjectId(projectId: string): Capability[] {
    return this.db.prepare('SELECT * FROM capabilities WHERE project_id = ?')
      .all(projectId)
      .map((row) => toCapability(row as Record<string, unknown>));
  }

  create(data: { projectId: string; name: string; description?: string }): Capability {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare(`INSERT INTO capabilities (id, project_id, name, description, self_evolve_enabled, self_evolve_min_score, self_evolve_max_revisions, self_evolve_cooldown_sec, self_evolve_target_env, self_evolve_dataset_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
      .run(id, data.projectId, data.name, data.description ?? '', 0, 0.7, 3, 900, '', '', now, now);
    return {
      id, projectId: data.projectId, name: data.name, description: data.description ?? '',
      createdAt: now, updatedAt: now,
      selfEvolveEnabled: false, selfEvolveMinScore: 0.7, selfEvolveMaxRevisions: 3,
      selfEvolveCooldownSec: 900, selfEvolveTargetEnv: '', selfEvolveDatasetId: '',
    };
  }

  update(id: string, data: Partial<Omit<Capability, 'id' | 'createdAt' | 'updatedAt'>>): Capability | null {
    const existing = this.findById(id);
    if (!existing) return null;
    const merged = { ...existing, ...data };
    this.db.prepare(`UPDATE capabilities SET project_id = ?, name = ?, description = ?, self_evolve_enabled = ?, self_evolve_min_score = ?, self_evolve_max_revisions = ?, self_evolve_cooldown_sec = ?, self_evolve_target_env = ?, self_evolve_dataset_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`)
      .run(merged.projectId, merged.name, merged.description, merged.selfEvolveEnabled ? 1 : 0, merged.selfEvolveMinScore, merged.selfEvolveMaxRevisions, merged.selfEvolveCooldownSec, merged.selfEvolveTargetEnv, merged.selfEvolveDatasetId, id);
    return merged;
  }
}
