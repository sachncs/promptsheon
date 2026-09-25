import type { TraceRun, TraceSpan } from '../repos/trace.js';
import type { TraceScore } from '../repos/trace-score.js';

export interface TraceListOptions {
  page?: number;
  pageSize?: number;
  environment?: string;
  status?: 'running' | 'success' | 'error';
  nameLike?: string;
  actorId?: string;
  fromTime?: string;
  toTime?: string;
}

export interface TraceRollupOptions {
  days?: number;
  environment?: string;
}

export interface TraceScoreSummaryOptions {
  days?: number;
  evaluator?: string;
}

export interface TraceAutoEvalOptions {
  judgeModel?: string;
  judgePrompt?: string;
}

export interface TraceStore {
  findByIdInOrg(id: string, organizationId: string): TraceRun | null;
  findSpansByRun(traceRunId: string): TraceSpan[];
  listByOrg(organizationId: string, options: TraceListOptions): { items: TraceRun[]; total: number };
  rollupByOrg(organizationId: string, options: TraceRollupOptions): Array<{
    day: string;
    tokens: number;
    cost: number;
    runs: number;
  }>;
}

export interface TraceScoreStore {
  listByRun(traceRunId: string): TraceScore[];
  summaryByOrg(organizationId: string, options: TraceScoreSummaryOptions): {
    totals: number;
    perEvaluator: Array<{ evaluator: string; count: number }>;
  };
}

export interface TraceEvaluator {
  run(traceRunId: string, options: TraceAutoEvalOptions): Promise<number>;
}

export interface TraceServiceDependencies {
  traces: TraceStore;
  scores: TraceScoreStore;
  evaluator: TraceEvaluator;
}

export interface TraceDetail {
  run: TraceRun;
  spans: TraceSpan[];
}

export interface TraceScores {
  run: TraceRun;
  items: TraceScore[];
  total: number;
}

/** Coordinates tenant-scoped trace reads and trace evaluation use cases. */
export class TraceService {
  constructor(private readonly deps: TraceServiceDependencies) {}

  list(organizationId: string, options: TraceListOptions): { items: TraceRun[]; total: number } {
    return this.deps.traces.listByOrg(organizationId, options);
  }

  rollup(organizationId: string, options: TraceRollupOptions) {
    return this.deps.traces.rollupByOrg(organizationId, options);
  }

  get(organizationId: string, traceRunId: string): TraceDetail | null {
    const run = this.deps.traces.findByIdInOrg(traceRunId, organizationId);
    if (!run) return null;
    return { run, spans: this.deps.traces.findSpansByRun(traceRunId) };
  }

  scores(organizationId: string, traceRunId: string): TraceScores | null {
    const run = this.deps.traces.findByIdInOrg(traceRunId, organizationId);
    if (!run) return null;
    const items = this.deps.scores.listByRun(traceRunId);
    return { run, items, total: items.length };
  }

  async autoEval(
    organizationId: string,
    traceRunId: string,
    options: TraceAutoEvalOptions,
  ): Promise<number | null> {
    if (!this.deps.traces.findByIdInOrg(traceRunId, organizationId)) return null;
    return this.deps.evaluator.run(traceRunId, options);
  }

  summary(organizationId: string, options: TraceScoreSummaryOptions) {
    return this.deps.scores.summaryByOrg(organizationId, options);
  }
}

export function createTraceService(deps: TraceServiceDependencies): TraceService {
  return new TraceService(deps);
}
