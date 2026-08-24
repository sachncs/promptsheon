import type Database from 'better-sqlite3';
import { randomUUID } from 'node:crypto';
import { BaseRepo, camelize } from './base.js';

export interface TraceRun {
  id: string;
  organizationId: string;
  actorId: string | null;
  executionId: string | null;
  sessionId: string | null;
  environment: string;
  name: string;
  startTime: string;
  endTime: string | null;
  status: 'running' | 'success' | 'error';
  attributes: Record<string, unknown>;
  totalTokens: number;
  totalCostUsd: number;
  model: string | null;
}

export interface TraceSpan {
  id: string;
  traceRunId: string;
  parentSpanId: string | null;
  name: string;
  kind: 'internal' | 'llm' | 'tool' | 'retrieval' | 'agent';
  startTime: string;
  endTime: string | null;
  status: 'ok' | 'error';
  attributes: Record<string, unknown>;
  model: string | null;
  promptTokens: number | null;
  completionTokens: number | null;
  totalTokens: number | null;
  costUsd: number | null;
  inputText: string | null;
  outputText: string | null;
}

interface TraceRunRow {
  id: string;
  organization_id: string;
  actor_id: string | null;
  execution_id: string | null;
  session_id: string | null;
  environment: string;
  name: string;
  start_time: string;
  end_time: string | null;
  status: string;
  attributes: string;
  total_tokens: number;
  total_cost_usd: number;
  model: string | null;
}

interface TraceSpanRow {
  id: string;
  trace_run_id: string;
  parent_span_id: string | null;
  name: string;
  kind: string;
  start_time: string;
  end_time: string | null;
  status: string;
  attributes: string;
  model: string | null;
  prompt_tokens: number | null;
  completion_tokens: number | null;
  total_tokens: number | null;
  cost_usd: number | null;
  input_text: string | null;
  output_text: string | null;
}

const TRUNCATE_TEXT_BYTES = 8 * 1024;

function truncate(text: string | null): string | null {
  if (text === null) return null;
  return text.length > TRUNCATE_TEXT_BYTES ? text.slice(0, TRUNCATE_TEXT_BYTES) + '…' : text;
}

function rowToRun(r: TraceRunRow): TraceRun {
  const camel = camelize(r as unknown as Record<string, unknown>);
  return {
    ...(camel as unknown as TraceRun),
    attributes: JSON.parse(r.attributes) as Record<string, unknown>,
    status: r.status as TraceRun['status'],
  };
}

function rowToSpan(s: TraceSpanRow): TraceSpan {
  const base = camelize(s as unknown as Record<string, unknown>);
  return {
    id: base['id'] as string,
    traceRunId: base['traceRunId'] as string,
    parentSpanId: base['parentSpanId'] as string | null,
    name: base['name'] as string,
    kind: base['kind'] as TraceSpan['kind'],
    startTime: base['startTime'] as string,
    endTime: base['endTime'] as string | null,
    status: base['status'] as TraceSpan['status'],
    attributes: JSON.parse(s.attributes) as Record<string, unknown>,
    model: base['model'] as string | null,
    promptTokens: base['promptTokens'] as number | null,
    completionTokens: base['completionTokens'] as number | null,
    totalTokens: base['totalTokens'] as number | null,
    costUsd: base['costUsd'] as number | null,
    inputText: base['inputText'] as string | null,
    outputText: base['outputText'] as string | null,
  };
}

export interface CreateTraceRunInput {
  organizationId: string;
  actorId?: string | null;
  executionId?: string | null;
  sessionId?: string | null;
  environment?: string;
  name: string;
  attributes?: Record<string, unknown>;
  model?: string | null;
}

export interface CreateTraceSpanInput {
  traceRunId: string;
  parentSpanId?: string | null;
  name: string;
  kind?: TraceSpan['kind'];
  attributes?: Record<string, unknown>;
  model?: string | null;
  promptTokens?: number | null;
  completionTokens?: number | null;
  totalTokens?: number | null;
  costUsd?: number | null;
  inputText?: string | null;
  outputText?: string | null;
  startTime?: string;
}

/**
 * TraceRepo — persistent storage for distributed traces. The
 * companion tracer in `observability/trace-context.ts` creates a
 * new TraceRun for each execution; child spans are appended via
 * `addSpan`; the run is finalised via `finalize` which aggregates
 * tokens / cost across child spans.
 */
export class TraceRepo extends BaseRepo<TraceRun> {
  constructor(db: Database.Database) {
    super(db, 'trace_runs');
  }

  startRun(input: CreateTraceRunInput): TraceRun {
    const id = randomUUID();
    const now = new Date().toISOString();
    this.db
      .prepare(
        `INSERT INTO trace_runs (id, organization_id, actor_id, execution_id, session_id,
          environment, name, start_time, status, attributes, model)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'running', ?, ?)`,
      )
      .run(
        id,
        input.organizationId,
        input.actorId ?? null,
        input.executionId ?? null,
        input.sessionId ?? null,
        input.environment ?? 'dev',
        input.name,
        now,
        JSON.stringify(input.attributes ?? {}),
        input.model ?? null,
      );
    return this.findById(id)!;
  }

  finalize(
    id: string,
    status: 'success' | 'error',
    totals?: { tokens?: number; costUsd?: number },
  ): void {
    const row = this.db
      .prepare(
        'SELECT SUM(COALESCE(total_tokens,0)) AS tokens, SUM(COALESCE(cost_usd,0)) AS cost FROM trace_spans WHERE trace_run_id = ?',
      )
      .get(id) as { tokens: number | null; cost: number | null } | undefined;
    this.db
      .prepare(
        `UPDATE trace_runs
         SET end_time = ?, status = ?, total_tokens = ?, total_cost_usd = ?
         WHERE id = ?`,
      )
      .run(
        new Date().toISOString(),
        status,
        totals?.tokens ?? row?.tokens ?? 0,
        totals?.costUsd ?? row?.cost ?? 0,
        id,
      );
  }

  addSpan(input: CreateTraceSpanInput): TraceSpan {
    const id = randomUUID();
    this.db
      .prepare(
        `INSERT INTO trace_spans (id, trace_run_id, parent_span_id, name, kind, start_time,
          status, attributes, model, prompt_tokens, completion_tokens, total_tokens, cost_usd,
          input_text, output_text)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(
        id,
        input.traceRunId,
        input.parentSpanId ?? null,
        input.name,
        input.kind ?? 'internal',
        input.startTime ?? new Date().toISOString(),
        'ok',
        JSON.stringify(input.attributes ?? {}),
        input.model ?? null,
        input.promptTokens ?? null,
        input.completionTokens ?? null,
        input.totalTokens ?? null,
        input.costUsd ?? null,
        truncate(input.inputText ?? null),
        truncate(input.outputText ?? null),
      );
    const row = this.db.prepare('SELECT * FROM trace_spans WHERE id = ?').get(id) as TraceSpanRow;
    return rowToSpan(row);
  }

  findSpansByRun(traceRunId: string): TraceSpan[] {
    const rows = this.db
      .prepare('SELECT * FROM trace_spans WHERE trace_run_id = ? ORDER BY start_time ASC')
      .all(traceRunId) as TraceSpanRow[];
    return rows.map(rowToSpan);
  }

  finishSpan(
    id: string,
    patch: {
      status?: 'ok' | 'error';
      endTime?: string;
      totalTokens?: number;
      costUsd?: number;
      outputText?: string;
    },
  ): void {
    this.db
      .prepare(
        `UPDATE trace_spans
         SET status = COALESCE(?, status),
             end_time = COALESCE(?, end_time),
             total_tokens = COALESCE(?, total_tokens),
             cost_usd = COALESCE(?, cost_usd),
             output_text = COALESCE(?, output_text)
         WHERE id = ?`,
      )
      .run(
        patch.status ?? null,
        patch.endTime ?? new Date().toISOString(),
        patch.totalTokens ?? null,
        patch.costUsd ?? null,
        patch.outputText ?? null,
        id,
      );
  }

  override findById(id: string): TraceRun | null {
    const row = this.db
      .prepare('SELECT * FROM trace_runs WHERE id = ?')
      .get(id) as TraceRunRow | undefined;
    return row ? rowToRun(row) : null;
  }

  /**
   * Paginated list of trace runs for an organization, newest first.
   * Supports filtering by environment, status, and a free-text
   * match against the run name.
   */
  listByOrg(
    organizationId: string,
    opts: {
      page?: number;
      pageSize?: number;
      environment?: string;
      status?: string;
      nameLike?: string;
      actorId?: string;
      fromTime?: string;
      toTime?: string;
    } = {},
  ): { items: TraceRun[]; total: number } {
    const page = opts.page ?? 1;
    const pageSize = Math.min(opts.pageSize ?? 25, 200);
    const conds: string[] = ['organization_id = ?'];
    const args: unknown[] = [organizationId];
    if (opts.environment) {
      conds.push('environment = ?');
      args.push(opts.environment);
    }
    if (opts.status) {
      conds.push('status = ?');
      args.push(opts.status);
    }
    if (opts.actorId) {
      conds.push('actor_id = ?');
      args.push(opts.actorId);
    }
    if (opts.nameLike) {
      conds.push('name LIKE ?');
      args.push(`%${opts.nameLike}%`);
    }
    if (opts.fromTime) {
      conds.push('start_time >= ?');
      args.push(opts.fromTime);
    }
    if (opts.toTime) {
      conds.push('start_time <= ?');
      args.push(opts.toTime);
    }
    const where = `WHERE ${conds.join(' AND ')}`;
    const total = (this.db.prepare(`SELECT COUNT(*) AS c FROM trace_runs ${where}`).get(...args) as {
      c: number;
    }).c;
    const rows = this.db
      .prepare(
        `SELECT * FROM trace_runs ${where}
         ORDER BY start_time DESC LIMIT ? OFFSET ?`,
      )
      .all(...args, pageSize, (page - 1) * pageSize) as TraceSpanRow[];
    const runs = rows.map((r) => {
      const camel = camelize(r as unknown as Record<string, unknown>);
      return {
        ...(camel as unknown as TraceRun),
        attributes: JSON.parse((r as unknown as { attributes: string }).attributes) as Record<string, unknown>,
        status: (r as unknown as { status: string }).status as TraceRun['status'],
      };
    });
    return { items: runs, total };
  }

  /**
   * Aggregate per-day rollup of token usage + cost for an
   * organization, used by the analytics page.
   */
  rollupByOrg(
    organizationId: string,
    opts: { days?: number; environment?: string } = {},
  ): Array<{ day: string; tokens: number; cost: number; runs: number }> {
    const days = opts.days ?? 30;
    const since = new Date(Date.now() - days * 86_400_000).toISOString();
    const envClause = opts.environment ? ' AND environment = ?' : '';
    const args: unknown[] = [organizationId, since];
    if (opts.environment) args.push(opts.environment);
    return this.db
      .prepare(
        `SELECT substr(start_time, 1, 10) AS day,
                SUM(total_tokens) AS tokens,
                SUM(total_cost_usd) AS cost,
                COUNT(*) AS runs
         FROM trace_runs
         WHERE organization_id = ? AND start_time >= ?${envClause}
         GROUP BY day
         ORDER BY day DESC`,
      )
      .all(...args) as Array<{ day: string; tokens: number; cost: number; runs: number }>;
  }
}
