import type Database from 'better-sqlite3';
import { randomUUID } from 'node:crypto';
import type { UserQuota } from '@promptsheon/shared';

interface QuotaRow {
  id: string;
  organization_id: string;
  user_id: string;
  label: string;
  daily_runs: number | null;
  daily_tokens: number | null;
  daily_cost_micros: number | null;
  enabled: number;
  created_at: string;
  updated_at: string;
}

function toQuota(row: QuotaRow): UserQuota {
  return {
    id: row.id,
    organizationId: row.organization_id,
    userId: row.user_id,
    label: row.label,
    dailyRuns: row.daily_runs,
    dailyTokens: row.daily_tokens,
    dailyCostMicros: row.daily_cost_micros,
    enabled: row.enabled === 1,
    createdAt: row.created_at,
    updatedAt: row.updated_at,
  };
}

export interface UserQuotaInput {
  organizationId: string;
  userId: string;
  label: string;
  dailyRuns?: number | null;
  dailyTokens?: number | null;
  dailyCostMicros?: number | null;
  enabled?: boolean;
}

export interface UserQuotaUsage {
  runs: number;
  tokens: number;
  costMicros: number;
}

/** Persists and reads organization-scoped per-user quota policies. */
export class UserQuotaRepo {
  constructor(private readonly db: Database.Database) {}

  listForOrg(organizationId: string): UserQuota[] {
    const rows = this.db.prepare('SELECT * FROM user_quotas WHERE organization_id = ? ORDER BY updated_at DESC').all(organizationId) as QuotaRow[];
    return rows.map(toQuota);
  }

  findById(id: string): UserQuota | null {
    const row = this.db.prepare('SELECT * FROM user_quotas WHERE id = ?').get(id) as QuotaRow | undefined;
    return row ? toQuota(row) : null;
  }

  findForUser(organizationId: string, userId: string): UserQuota | null {
    const row = this.db.prepare('SELECT * FROM user_quotas WHERE organization_id = ? AND user_id = ?').get(organizationId, userId) as QuotaRow | undefined;
    return row ? toQuota(row) : null;
  }

  usage(organizationId: string, userId: string, now = new Date()): UserQuotaUsage {
    const dayStart = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate())).toISOString();
    const jobs = this.db.prepare(`SELECT COUNT(*) AS runs FROM execution_jobs WHERE organization_id = ? AND actor_id = ? AND created_at >= ?`).get(organizationId, userId, dayStart) as { runs: number };
    const traces = this.db.prepare(`SELECT COALESCE(SUM(total_tokens), 0) AS tokens, COALESCE(SUM(total_cost_usd), 0) AS cost FROM trace_runs WHERE organization_id = ? AND actor_id = ? AND start_time >= ?`).get(organizationId, userId, dayStart) as { tokens: number; cost: number };
    return { runs: jobs.runs, tokens: traces.tokens, costMicros: Math.ceil(traces.cost * 1_000_000) };
  }

  create(input: UserQuotaInput): UserQuota {
    const id = randomUUID();
    const now = new Date().toISOString();
    this.db.prepare(`
      INSERT INTO user_quotas (id, organization_id, user_id, label, daily_runs, daily_tokens, daily_cost_micros, enabled, created_at, updated_at)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `).run(id, input.organizationId, input.userId, input.label, input.dailyRuns ?? null, input.dailyTokens ?? null, input.dailyCostMicros ?? null, input.enabled ?? true ? 1 : 0, now, now);
    return this.findById(id)!;
  }

  update(id: string, fields: Omit<Partial<UserQuotaInput>, 'organizationId' | 'userId'>): UserQuota | null {
    const existing = this.findById(id);
    if (!existing) return null;
    const next = {
      label: fields.label ?? existing.label,
      dailyRuns: fields.dailyRuns === undefined ? existing.dailyRuns : fields.dailyRuns,
      dailyTokens: fields.dailyTokens === undefined ? existing.dailyTokens : fields.dailyTokens,
      dailyCostMicros: fields.dailyCostMicros === undefined ? existing.dailyCostMicros : fields.dailyCostMicros,
      enabled: fields.enabled ?? existing.enabled,
    };
    this.db.prepare(`UPDATE user_quotas SET label = ?, daily_runs = ?, daily_tokens = ?, daily_cost_micros = ?, enabled = ?, updated_at = ? WHERE id = ?`).run(next.label, next.dailyRuns, next.dailyTokens, next.dailyCostMicros, next.enabled ? 1 : 0, new Date().toISOString(), id);
    return this.findById(id);
  }

  delete(id: string): boolean {
    return this.db.prepare('DELETE FROM user_quotas WHERE id = ?').run(id).changes > 0;
  }
}
