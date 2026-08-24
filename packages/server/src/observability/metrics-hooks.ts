import type Database from 'better-sqlite3';
import type { AfterInvocationEvent } from '@strands-agents/sdk';
import type { ManifestRepo } from '../repos/manifest.js';
import type { TraceRepo } from '../repos/trace.js';

export interface MetricsHookContext {
  executionId: string;
  manifestHash: string;
  manifestRepo: ManifestRepo;
  traceRepo?: TraceRepo;
  traceRunId?: string;
  /**
   * Optional wall-clock start time of the invocation (ms since epoch).
   * When supplied, the hook stamps `startedAt` from this value and
   * computes `latencyMs = now - startedAt` so the persisted row has
   * a real duration instead of zero.
   */
  invocationStartedAt?: number;
}

/**
 * Create a Strands hook callback that captures per-invocation metrics
 * (tokens, latency) and persists them as a `node_runs` row.
 *
 * The hook reads `event.agent.metrics.accumulatedUsage` for token counts
 * and `event.invocationState` for execution correlation. It is intentionally
 * tolerant of missing metrics (writes a zero-row rather than throwing) so
 * the executor loop continues even when an SDK upgrade changes the shape
 * of `AgentMetrics`.
 */
export function createMetricsHook(ctx: MetricsHookContext): (event: AfterInvocationEvent) => void | Promise<void> {
  return async (event: AfterInvocationEvent) => {
    try {
      const metrics = event.agent.metrics;
      const usage = metrics?.accumulatedUsage;
      const nodeId = event.agent.id || 'unknown';

      const totalTokens = usage?.totalTokens ?? 0;
      const promptTokens = usage?.inputTokens ?? 0;
      const completionTokens = usage?.outputTokens ?? 0;

      const endedAt = Date.now();
      const startedAt = ctx.invocationStartedAt ?? endedAt;
      const latencyMs = Math.max(0, endedAt - startedAt);

      ctx.manifestRepo.recordNodeRun({
        manifestHash: ctx.manifestHash,
        nodeId,
        executionId: ctx.executionId,
        startedAt: new Date(startedAt).toISOString(),
        endedAt: new Date(endedAt).toISOString(),
        latencyMs,
        costUsd: totalTokens ? (totalTokens / 1000) * 0.00003 : 0,
        promptTokens,
        completionTokens,
        totalTokens,
        error: '',
        status: 'completed',
      });

      // Mirror to the trace store so /api/traces can show per-node
      // spans for the same execution. The trace_run_id is shared
      // across all nodes of the same execution; we add a span under
      // it on every invocation. Errors are swallowed — span
      // persistence is best-effort.
      if (ctx.traceRepo && ctx.traceRunId) {
        try {
          ctx.traceRepo.addSpan({
            traceRunId: ctx.traceRunId,
            name: nodeId,
            kind: 'agent',
            startTime: new Date(startedAt).toISOString(),
            attributes: {
              manifestHash: ctx.manifestHash,
              executionId: ctx.executionId,
            },
            model: undefined,
            promptTokens,
            completionTokens,
            totalTokens: totalTokens || undefined,
            costUsd: totalTokens ? (totalTokens / 1000) * 0.00003 : undefined,
            inputText: undefined,
            outputText: undefined,
          });
          const span = ctx.traceRepo
            .findSpansByRun(ctx.traceRunId)
            .reverse()
            .find((s) => s.name === nodeId);
          if (span) {
            ctx.traceRepo.finishSpan(span.id, {
              endTime: new Date(endedAt).toISOString(),
              totalTokens: totalTokens || undefined,
              costUsd: totalTokens ? (totalTokens / 1000) * 0.00003 : undefined,
            });
          }
        } catch {
          // never let span persistence break execution
        }
      }
    } catch {
      // never let metrics persistence break execution
    }
  };
}

export interface NodeRunRow {
  id: string;
  manifestHash: string;
  nodeId: string;
  executionId: string | null;
  startedAt: string;
  endedAt: string | null;
  latencyMs: string | null;
  costUsd: number;
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
  error: string;
  status: string;
}

/**
 * Convenience reader: list all `node_runs` for an execution in
 * chronological order. Returns an empty array when the table is empty
 * or the execution is unknown.
 */
export function listNodeRuns(db: Database.Database, executionId: string): NodeRunRow[] {
  const rows = db
    .prepare(
      `SELECT id, manifest_hash, node_id, execution_id, started_at, ended_at,
              latency_ms, cost_usd, prompt_tokens, completion_tokens, total_tokens,
              error, status
       FROM node_runs WHERE execution_id = ? ORDER BY started_at ASC`,
    )
    .all(executionId) as Array<{
    id: string;
    manifest_hash: string;
    node_id: string;
    execution_id: string | null;
    started_at: string;
    ended_at: string | null;
    latency_ms: string | null;
    cost_usd: number;
    prompt_tokens: number;
    completion_tokens: number;
    total_tokens: number;
    error: string;
    status: string;
  }>;

  return rows.map((r) => ({
    id: r.id,
    manifestHash: r.manifest_hash,
    nodeId: r.node_id,
    executionId: r.execution_id,
    startedAt: r.started_at,
    endedAt: r.ended_at,
    latencyMs: r.latency_ms,
    costUsd: r.cost_usd,
    promptTokens: r.prompt_tokens,
    completionTokens: r.completion_tokens,
    totalTokens: r.total_tokens,
    error: r.error,
    status: r.status,
  }));
}