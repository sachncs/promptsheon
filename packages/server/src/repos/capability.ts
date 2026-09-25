import type { Capability } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { z } from 'zod';
import { BaseRepo, type Paginated } from './base.js';

const CapabilityRowSchema = z.object({
  id: z.string(),
  project_id: z.string(),
  name: z.string(),
  description: z.string().nullish().transform((value) => value ?? ''),
  created_at: z.string(),
  updated_at: z.string(),
  self_evolve_enabled: z.union([z.number().int(), z.boolean()]),
  self_evolve_min_score: z.number().min(0).max(1),
  self_evolve_max_revisions: z.number().int().nonnegative(),
  self_evolve_cooldown_sec: z.number().int().nonnegative(),
  self_evolve_target_env: z.string(),
  self_evolve_dataset_id: z.string().nullish().transform((value) => value ?? ''),
});

const CountSchema = z.object({ count: z.number().int().nonnegative() });

function toCapability(row: unknown): Capability {
  const value = CapabilityRowSchema.parse(row);
  return {
    id: value.id,
    projectId: value.project_id,
    name: value.name,
    description: value.description,
    createdAt: value.created_at,
    updatedAt: value.updated_at,
    selfEvolveEnabled: typeof value.self_evolve_enabled === 'boolean'
      ? value.self_evolve_enabled
      : value.self_evolve_enabled === 1,
    selfEvolveMinScore: value.self_evolve_min_score,
    selfEvolveMaxRevisions: value.self_evolve_max_revisions,
    selfEvolveCooldownSec: value.self_evolve_cooldown_sec,
    selfEvolveTargetEnv: value.self_evolve_target_env,
    selfEvolveDatasetId: value.self_evolve_dataset_id,
  };
}

export class CapabilityRepo extends BaseRepo<Capability> {
  constructor(db: Database.Database) {
    super(db, 'capabilities');
  }

  override findById(id: string): Capability | null {
    const row = this.db.prepare('SELECT * FROM capabilities WHERE id = ?').get(id);
    return row ? toCapability(row) : null;
  }

  override findMany(opts: { page: number; pageSize: number }): Paginated<Capability> {
    const total = CountSchema.parse(this.db.prepare('SELECT COUNT(*) AS count FROM capabilities').get()).count;
    const rows = this.db.prepare('SELECT * FROM capabilities LIMIT ? OFFSET ?')
      .all(opts.pageSize, (opts.page - 1) * opts.pageSize);
    return { items: rows.map(toCapability), total };
  }

  findByProjectId(projectId: string): Capability[] {
    return this.db.prepare('SELECT * FROM capabilities WHERE project_id = ?')
      .all(projectId)
      .map(toCapability);
  }

  findByProjectIdInOrg(projectId: string, organizationId: string): Capability[] {
    return this.db.prepare(
      `SELECT c.* FROM capabilities c JOIN projects p ON p.id = c.project_id
       JOIN workspaces w ON w.id = p.workspace_id
       WHERE c.project_id = ? AND w.org_id = ? ORDER BY c.created_at DESC`,
    ).all(projectId, organizationId).map(toCapability);
  }

  findByIdInOrg(id: string, organizationId: string): Capability | null {
    const row = this.db.prepare(
      `SELECT c.* FROM capabilities c JOIN projects p ON p.id = c.project_id
       JOIN workspaces w ON w.id = p.workspace_id
       WHERE c.id = ? AND w.org_id = ?`,
    ).get(id, organizationId);
    return row ? toCapability(row) : null;
  }

  findManyInOrg(organizationId: string, opts: { page: number; pageSize: number }): Paginated<Capability> {
    const joins = ' FROM capabilities c JOIN projects p ON p.id = c.project_id JOIN workspaces w ON w.id = p.workspace_id WHERE w.org_id = ?';
    const total = CountSchema.parse(this.db.prepare(`SELECT COUNT(*) AS count${joins}`).get(organizationId)).count;
    const rows = this.db.prepare(`SELECT c.*${joins} ORDER BY c.created_at DESC LIMIT ? OFFSET ?`)
      .all(organizationId, opts.pageSize, (opts.page - 1) * opts.pageSize);
    return { items: rows.map(toCapability), total };
  }

  createInOrg(data: { projectId: string; name: string; description?: string }, organizationId: string): Capability | null {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    const result = this.db.prepare(`
      INSERT INTO capabilities (id, project_id, name, description, self_evolve_enabled, self_evolve_min_score, self_evolve_max_revisions, self_evolve_cooldown_sec, self_evolve_target_env, self_evolve_dataset_id, created_at, updated_at)
      SELECT ?, ?, ?, ?, 0, 0.7, 3, 900, '', '', ?, ?
      WHERE EXISTS (SELECT 1 FROM projects p JOIN workspaces w ON w.id = p.workspace_id WHERE p.id = ? AND w.org_id = ?)
    `).run(id, data.projectId, data.name, data.description ?? '', now, now, data.projectId, organizationId);
    if (result.changes === 0) return null;
    return this.findByIdInOrg(id, organizationId);
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

  updateInOrg(id: string, organizationId: string, data: Partial<Omit<Capability, 'id' | 'createdAt' | 'updatedAt'>>): Capability | null {
    const existing = this.findByIdInOrg(id, organizationId);
    if (!existing) return null;
    const merged = { ...existing, ...data };
    this.db.prepare(`UPDATE capabilities SET project_id = ?, name = ?, description = ?, self_evolve_enabled = ?, self_evolve_min_score = ?, self_evolve_max_revisions = ?, self_evolve_cooldown_sec = ?, self_evolve_target_env = ?, self_evolve_dataset_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND EXISTS (SELECT 1 FROM projects p JOIN workspaces w ON w.id = p.workspace_id WHERE p.id = capabilities.project_id AND w.org_id = ?)`)
      .run(merged.projectId, merged.name, merged.description, merged.selfEvolveEnabled ? 1 : 0, merged.selfEvolveMinScore, merged.selfEvolveMaxRevisions, merged.selfEvolveCooldownSec, merged.selfEvolveTargetEnv, merged.selfEvolveDatasetId, id, organizationId);
    return merged;
  }

  deleteInOrg(id: string, organizationId: string): boolean {
    return this.db.prepare(`DELETE FROM capabilities WHERE id = ? AND EXISTS (SELECT 1 FROM projects p JOIN workspaces w ON w.id = p.workspace_id WHERE p.id = capabilities.project_id AND w.org_id = ?)`)
      .run(id, organizationId).changes > 0;
  }
}
