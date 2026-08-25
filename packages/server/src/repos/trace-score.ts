import type Database from 'better-sqlite3';
import { randomUUID } from 'node:crypto';
import { BaseRepo } from './base.js';

export interface TraceScore {
  id: string;
  traceRunId: string;
  executionId: string | null;
  evaluator: string;
  name: string;
  value: number | null;
  label: string | null;
  rationale: string | null;
  createdAt: string;
}

export interface CreateTraceScoreInput {
  traceRunId: string;
  executionId?: string | null;
  evaluator: string;
  name: string;
  value?: number | null;
  label?: string | null;
  rationale?: string | null;
}

interface TraceScoreRow {
  id: string;
  trace_run_id: string;
  execution_id: string | null;
  evaluator: string;
  name: string;
  value: number | null;
  label: string | null;
  rationale: string | null;
  created_at: string;
}

function rowToScore(r: TraceScoreRow): TraceScore {
  return {
    id: r.id,
    traceRunId: r.trace_run_id,
    executionId: r.execution_id,
    evaluator: r.evaluator,
    name: r.name,
    value: r.value,
    label: r.label,
    rationale: r.rationale,
    createdAt: r.created_at,
  };
}

/**
 * TraceScoreRepo — eval results attached to trace runs. Built-in
 * evaluators (latency-budget, error-rate, output-shape) write
 * here; user-defined LLM-as-judge evaluators use the same store.
 */
export class TraceScoreRepo extends BaseRepo<TraceScore> {
  constructor(db: Database.Database) {
    super(db, 'trace_scores');
  }

  record(input: CreateTraceScoreInput): TraceScore {
    const id = randomUUID();
    this.db
      .prepare(
        `INSERT INTO trace_scores (id, trace_run_id, execution_id, evaluator, name, value, label, rationale, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(
        id,
        input.traceRunId,
        input.executionId ?? null,
        input.evaluator,
        input.name,
        input.value ?? null,
        input.label ?? null,
        input.rationale ?? null,
        new Date().toISOString(),
      );
    const row = this.db
      .prepare('SELECT * FROM trace_scores WHERE id = ?')
      .get(id) as TraceScoreRow;
    return rowToScore(row);
  }

  listByRun(traceRunId: string): TraceScore[] {
    const rows = this.db
      .prepare('SELECT * FROM trace_scores WHERE trace_run_id = ? ORDER BY created_at ASC')
      .all(traceRunId) as TraceScoreRow[];
    return rows.map(rowToScore);
  }

  /**
   * Aggregate summary across the org for a date range — used by
   * the analytics surface. Returns count of scores + the share
   * that fell into each label (when labels are set).
   */
  summaryByOrg(
    organizationId: string,
    opts: { days?: number; evaluator?: string } = {},
  ): { totals: number; perEvaluator: Array<{ evaluator: string; count: number }> } {
    const days = opts.days ?? 7;
    const since = new Date(Date.now() - days * 86_400_000).toISOString();
    const evClause = opts.evaluator ? ' AND s.evaluator = ?' : '';
    const args: unknown[] = [organizationId, since];
    if (opts.evaluator) args.push(opts.evaluator);
    const totals = (
      this.db
        .prepare(
          `SELECT COUNT(*) AS count FROM trace_scores s
           JOIN trace_runs r ON r.id = s.trace_run_id
           WHERE r.organization_id = ? AND s.created_at >= ?${evClause}`,
        )
        .get(...args) as { count: number }
    ).count;
    const perEval = this.db
      .prepare(
        `SELECT s.evaluator AS evaluator, COUNT(*) AS count FROM trace_scores s
         JOIN trace_runs r ON r.id = s.trace_run_id
         WHERE r.organization_id = ? AND s.created_at >= ?${evClause}
         GROUP BY s.evaluator`,
      )
      .all(...args) as Array<{ evaluator: string; count: number }>;
    return { totals, perEvaluator: perEval };
  }
}
