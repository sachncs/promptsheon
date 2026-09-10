import type Database from 'better-sqlite3';
import { BaseRepo, camelize } from './base.js';

export interface UserDailyUsage {
  day: string;
  runs: number;
  tokens: number;
  cost: number;
}

export interface UserRollup {
  actorId: string;
  runs: number;
  tokens: number;
  cost: number;
  days: number;
}

interface TraceRunWithActorRow {
  actor_id: string | null;
  total_tokens: number;
  total_cost_usd: number;
  start_time: string;
}

/**
 * UserAnalyticsRepo — per-user (actor) rollups over trace_runs.
 * Drives the per-tenant analytics page ("user X used N tokens
 * today") and per-user rate-limit dashboards.
 *
 * Stats are computed lazily from trace_runs at read-time — there
 * is no separate user_totals table. That's fine for the data
 * volumes we expect (thousands of runs/day per org) but if
 * per-user aggregation becomes a hot path, add a materialised
 * daily-rollup table.
 */
export class UserAnalyticsRepo extends BaseRepo<never> {
  constructor(db: Database.Database) {
    super(db, 'trace_runs');
  }

  /**
   * Per-day usage for a single user across the last N days.
   */
  perDay(actorId: string, days = 30): UserDailyUsage[] {
    const since = new Date(Date.now() - days * 86_400_000).toISOString();
    return this.db
      .prepare(
        `SELECT substr(start_time, 1, 10) AS day,
                COUNT(*) AS runs,
                SUM(total_tokens) AS tokens,
                SUM(total_cost_usd) AS cost
         FROM trace_runs
         WHERE actor_id = ? AND start_time >= ?
         GROUP BY day
         ORDER BY day DESC`,
      )
      .all(actorId, since)
      .map((r: unknown) => {
        const row = r as { day: string; runs: number; tokens: number; cost: number };
        return {
          day: row.day,
          runs: row.runs,
          tokens: row.tokens ?? 0,
          cost: row.cost ?? 0,
        };
      });
  }

  /**
   * One row per active user in the org, sorted by tokens
   * descending. Used by the analytics page "top consumers" tile.
   */
  leaderboardByOrg(
    organizationId: string,
    opts: { days?: number; limit?: number } = {},
  ): UserRollup[] {
    const days = opts.days ?? 30;
    const limit = Math.min(opts.limit ?? 25, 200);
    const since = new Date(Date.now() - days * 86_400_000).toISOString();
    return this.db
      .prepare(
        `SELECT actor_id,
                COUNT(*) AS runs,
                SUM(total_tokens) AS tokens,
                SUM(total_cost_usd) AS cost,
                COUNT(DISTINCT substr(start_time, 1, 10)) AS days
         FROM trace_runs
         WHERE organization_id = ? AND start_time >= ?
           AND actor_id IS NOT NULL
         GROUP BY actor_id
         ORDER BY tokens DESC
         LIMIT ?`,
      )
      .all(organizationId, since, limit)
      .map((r: unknown) => {
        const row = r as TraceRunWithActorRow & { runs: number; tokens: number; cost: number; days: number };
        return {
          actorId: row.actor_id ?? 'unscoped',
          runs: row.runs ?? 0,
          tokens: row.tokens ?? 0,
          cost: row.cost ?? 0,
          days: row.days ?? 0,
        };
      });
  }

  /**
   * Per-org org-level aggregate: total tokens, total cost, total
   * runs over the window. Plus a breakdown of active days.
   */
  orgTotals(
    organizationId: string,
    days = 30,
  ): { tokens: number; cost: number; runs: number; activeDays: number } {
    const since = new Date(Date.now() - days * 86_400_000).toISOString();
    const row = this.db
      .prepare(
        `SELECT COUNT(*) AS runs,
                COALESCE(SUM(total_tokens),0) AS tokens,
                COALESCE(SUM(total_cost_usd),0) AS cost,
                COUNT(DISTINCT substr(start_time, 1, 10)) AS active_days
         FROM trace_runs
         WHERE organization_id = ? AND start_time >= ?`,
      )
      .get(organizationId, since) as
      | { runs: number; tokens: number; cost: number; active_days: number }
      | undefined;
    return {
      runs: row?.runs ?? 0,
      tokens: row?.tokens ?? 0,
      cost: row?.cost ?? 0,
      activeDays: row?.active_days ?? 0,
    };
  }

  /**
   * Audit the trace_runs table is reachable and BaseRepo's
   * camelize doesn't blow up on a column with snake_case keys.
   * Smoke test for the BaseRepo.findById fix.
   */
  static camelizeSmokeTest(row: Record<string, unknown>): Record<string, unknown> {
    return camelize(row);
  }
}
