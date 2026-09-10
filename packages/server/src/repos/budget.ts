import type Database from 'better-sqlite3';
import { randomUUID } from 'node:crypto';
import type { BudgetPeriod, CostBudget } from '@promptsheon/shared';

interface BudgetRow {
  id: string;
  organization_id: string;
  label: string;
  period: BudgetPeriod;
  limit_micros: number;
  alert_threshold: number;
  enabled: number;
  created_at: string;
  updated_at: string;
  last_alerted_at: string | null;
}

function rowToBudget(r: BudgetRow): CostBudget {
  return {
    id: r.id,
    organizationId: r.organization_id,
    label: r.label,
    period: r.period,
    limitMicros: r.limit_micros,
    alertThreshold: r.alert_threshold,
    enabled: r.enabled === 1,
    createdAt: r.created_at,
    updatedAt: r.updated_at,
    lastAlertedAt: r.last_alerted_at,
  };
}

export interface BudgetInput {
  organizationId: string;
  label: string;
  period?: BudgetPeriod;
  limitMicros: number;
  alertThreshold?: number;
  enabled?: boolean;
}

/**
 * CRUD over the `cost_budgets` table. `updateLastAlerted` is the
 * cooldown mechanism used by the forecast service to avoid
 * webhook storms when the dashboard is hit repeatedly.
 */
export class CostBudgetRepo {
  constructor(private db: Database.Database) {}

  listForOrg(orgId: string): CostBudget[] {
    const rows = this.db
      .prepare(`SELECT * FROM cost_budgets WHERE organization_id = ? ORDER BY created_at DESC`)
      .all(orgId) as BudgetRow[];
    return rows.map(rowToBudget);
  }

  findById(id: string): CostBudget | null {
    const row = this.db.prepare(`SELECT * FROM cost_budgets WHERE id = ?`).get(id) as BudgetRow | undefined;
    return row ? rowToBudget(row) : null;
  }

  create(input: BudgetInput): CostBudget {
    const id = randomUUID();
    const now = new Date().toISOString();
    this.db
      .prepare(
        `INSERT INTO cost_budgets (id, organization_id, label, period, limit_micros, alert_threshold, enabled, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(
        id,
        input.organizationId,
        input.label,
        input.period ?? 'monthly',
        input.limitMicros,
        input.alertThreshold ?? 0.8,
        (input.enabled ?? true) ? 1 : 0,
        now,
        now,
      );
    return this.findById(id)!;
  }

  update(
    id: string,
    fields: {
      label?: string;
      period?: BudgetPeriod;
      limitMicros?: number;
      alertThreshold?: number;
      enabled?: boolean;
    },
  ): CostBudget | null {
    const existing = this.findById(id);
    if (!existing) return null;
    const next = {
      label: fields.label ?? existing.label,
      period: fields.period ?? existing.period,
      limitMicros: fields.limitMicros ?? existing.limitMicros,
      alertThreshold: fields.alertThreshold ?? existing.alertThreshold,
      enabled: fields.enabled ?? existing.enabled,
    };
    this.db
      .prepare(
        `UPDATE cost_budgets
         SET label = ?, period = ?, limit_micros = ?, alert_threshold = ?, enabled = ?, updated_at = ?
         WHERE id = ?`,
      )
      .run(
        next.label,
        next.period,
        next.limitMicros,
        next.alertThreshold,
        next.enabled ? 1 : 0,
        new Date().toISOString(),
        id,
      );
    return this.findById(id);
  }

  delete(id: string): boolean {
    const result = this.db.prepare(`DELETE FROM cost_budgets WHERE id = ?`).run(id);
    return result.changes > 0;
  }

  updateLastAlerted(id: string, ts: string): void {
    this.db.prepare(`UPDATE cost_budgets SET last_alerted_at = ? WHERE id = ?`).run(ts, id);
  }

  /**
   * Latest forecast snapshot for an org, or null when none
   * exists. The forecast service is the only writer; this is a
   * read-only convenience for the dashboard.
   */
  latestForecast(orgId: string): { id: string; computedAt: string } | null {
    const row = this.db
      .prepare(
        `SELECT id, computed_at AS computedAt
         FROM cost_forecast_snapshots
         WHERE organization_id = ?
         ORDER BY computed_at DESC
         LIMIT 1`,
      )
      .get(orgId) as { id: string; computedAt: string } | undefined;
    return row ?? null;
  }

  insertForecastSnapshot(snapshot: {
    id: string;
    organizationId: string;
    periodStart: string;
    periodEnd: string;
    spendMicros: number;
    projectedMicros: number;
    bandLowMicros: number;
    bandHighMicros: number;
    windowDays: number;
    computedAt: string;
  }): void {
    this.db
      .prepare(
        `INSERT INTO cost_forecast_snapshots (
          id, organization_id, period_start, period_end,
          spend_micros, projected_micros, band_low_micros, band_high_micros,
          window_days, computed_at
         ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(
        snapshot.id,
        snapshot.organizationId,
        snapshot.periodStart,
        snapshot.periodEnd,
        snapshot.spendMicros,
        snapshot.projectedMicros,
        snapshot.bandLowMicros,
        snapshot.bandHighMicros,
        snapshot.windowDays,
        snapshot.computedAt,
      );
  }
}