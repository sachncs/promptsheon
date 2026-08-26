import { randomUUID } from 'node:crypto';
import type Database from 'better-sqlite3';
import type { TraceRepo, TraceSpan, TraceRun } from '../repos/trace.js';

/**
 * Tracer — a thin facade over TraceRepo that handles span
 * parent/child relationships automatically. Open one Tracer per
 * trace root; nest child tracers via `.span(...)`. Each call to
 * `.span` returns a `Span` whose `.end()` records duration +
 * optional LLM/cost metadata.
 *
 * Span timing is wall-clock; cost/tokens are set explicitly when
 * the caller knows them (typically right before .end).
 */
export interface LlmCallMetadata {
  model: string;
  promptTokens?: number;
  completionTokens?: number;
  totalTokens?: number;
  costUsd?: number;
  inputText?: string;
  outputText?: string;
}

export class Span {
  readonly id: string;
  readonly start: number;
  constructor(
    id: string,
    public readonly name: string,
    public readonly parentId: string | null,
    public readonly tracer: Tracer,
  ) {
    this.id = id;
    this.start = Date.now();
  }

  setLlmCall(meta: LlmCallMetadata): void {
    this.llmMeta = meta;
  }

  setStatus(status: 'ok' | 'error'): void {
    this.status = status;
  }

  setAttribute(key: string, value: unknown): void {
    if (!this.attributes[key]) this.attributes[key] = value;
  }

  end(): TraceSpan {
    const endTime = new Date().toISOString();
    const meta = this.llmMeta;
    const totalTokens = meta?.totalTokens ?? (meta?.promptTokens ?? 0) + (meta?.completionTokens ?? 0);
    this.tracer.repo.addSpan({
      traceRunId: this.tracer.run.id,
      parentSpanId: this.parentId,
      name: this.name,
      kind: meta ? 'llm' : 'internal',
      attributes: this.attributes,
      model: meta?.model ?? null,
      promptTokens: meta?.promptTokens ?? null,
      completionTokens: meta?.completionTokens ?? null,
      totalTokens: totalTokens || null,
      costUsd: meta?.costUsd ?? null,
      inputText: meta?.inputText ?? null,
      outputText: meta?.outputText ?? null,
      startTime: new Date(this.start).toISOString(),
    });
    this.tracer.repo.finishSpan(this.id, {
      status: this.status === 'error' ? 'error' : 'ok',
      endTime,
      totalTokens: totalTokens || undefined,
      costUsd: meta?.costUsd,
      outputText: meta?.outputText,
    });
    return this.tracer.repo.findSpansByRun(this.tracer.run.id).find((s) => s.id === this.id)!;
  }

  private llmMeta?: LlmCallMetadata;
  private status: 'ok' | 'error' = 'ok';
  private readonly attributes: Record<string, unknown> = {};
}

export class Tracer {
  readonly run: TraceRun;
  readonly repo: TraceRepo;

  constructor(run: TraceRun, repo: TraceRepo) {
    this.run = run;
    this.repo = repo;
  }

  /**
   * Open a child span. Caller must invoke `.end()` on the returned
   * Span when the operation completes.
   */
  span(name: string, kind?: 'internal' | 'llm' | 'tool' | 'retrieval' | 'agent'): Span {
    const span = new Span(randomUUID(), name, null, this);
    // Pre-create with current time so parent/child ordering is
    // preserved on SELECT.
    this.repo.addSpan({
      traceRunId: this.run.id,
      name,
      kind: kind ?? 'internal',
      startTime: new Date(span.start).toISOString(),
    });
    // Mutate the id so end() persists to the right row.
    (span as unknown as { id: string }).id = this.repo.findSpansByRun(this.run.id).find((s) => s.name === name)?.id ?? span.id;
    return span;
  }

  finalize(status: 'success' | 'error' = 'success'): void {
    this.repo.finalize(this.run.id, status);
  }
}

/**
 * Convenience constructor: open a new trace and return a Tracer
 * bound to it. Caller is responsible for calling .finalize().
 */
export function startTrace(
  repo: TraceRepo,
  input: {
    organizationId: string;
    actorId?: string | null;
    executionId?: string | null;
    sessionId?: string | null;
    environment?: string;
    name: string;
    model?: string | null;
    attributes?: Record<string, unknown>;
  },
): Tracer {
  const run = repo.startRun(input);
  return new Tracer(run, repo);
}

/**
 * _db arg is unused — kept for symmetry with other utilities
 * that may want to wrap open + start. Currently the only reason
 * to import this file is to attach a Tracer to a running
 * execution; nothing else needs the bare db handle.
 */
export type _db = Database.Database;
