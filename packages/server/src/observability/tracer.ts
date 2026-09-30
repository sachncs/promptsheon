import { randomUUID } from 'node:crypto';
import type Database from 'better-sqlite3';
import type { TraceRepo, TraceSpan, TraceRun } from '../repos/trace.js';
import {
  context as otelContext,
  SpanKind,
  SpanStatusCode,
  trace as otelTrace,
  type Attributes,
  type Span as OtelSpan,
  type Tracer as OtelTracer,
} from '@opentelemetry/api';

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
    private readonly otelSpan: OtelSpan,
  ) {
    this.id = id;
    this.start = Date.now();
  }

  setLlmCall(meta: LlmCallMetadata): void {
    this.llmMeta = meta;
  }

  setStatus(status: 'ok' | 'error'): void {
    this.status = status;
    this.otelSpan.setStatus(status === 'error' ? { code: SpanStatusCode.ERROR } : { code: SpanStatusCode.OK });
  }

  setAttribute(key: string, value: unknown): void {
    if (!this.attributes[key]) this.attributes[key] = value;
    const attribute = toOtelAttribute(value);
    if (attribute !== undefined) this.otelSpan.setAttribute(key, attribute);
  }

  end(): TraceSpan {
    if (this.ended) return this.tracer.repo.findSpansByRun(this.tracer.run.id).find((s) => s.id === this.id)!;
    this.ended = true;
    const endTime = new Date().toISOString();
    const meta = this.llmMeta;
    const totalTokens = meta?.totalTokens ?? (meta?.promptTokens ?? 0) + (meta?.completionTokens ?? 0);
    this.tracer.repo.addSpan({
      id: this.id,
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
    this.otelSpan.end();
    return this.tracer.repo.findSpansByRun(this.tracer.run.id).find((s) => s.id === this.id)!;
  }

  private llmMeta?: LlmCallMetadata;
  private status: 'ok' | 'error' = 'ok';
  private ended = false;
  private readonly attributes: Record<string, unknown> = {};
}

export class Tracer {
  readonly run: TraceRun;
  readonly repo: TraceRepo;

  constructor(run: TraceRun, repo: TraceRepo, private readonly otelTracer: OtelTracer = otelTrace.getTracer('promptsheon')) {
    this.run = run;
    this.repo = repo;
    this.otelRoot = this.otelTracer.startSpan(run.name, {
      attributes: toOtelAttributes(run.attributes),
    });
  }

  /**
   * Open a child span. Caller must invoke `.end()` on the returned
   * Span when the operation completes.
   */
  span(name: string, kind?: 'internal' | 'llm' | 'tool' | 'retrieval' | 'agent'): Span {
    const parentContext = otelTrace.setSpan(otelContext.active(), this.otelRoot);
    const otelSpan = this.otelTracer.startSpan(name, {
      kind: toOtelSpanKind(kind),
    }, parentContext);
    const span = new Span(randomUUID(), name, null, this, otelSpan);
    // Pre-create with current time so parent/child ordering is
    // preserved on SELECT.
    this.repo.addSpan({
      traceRunId: this.run.id,
      name,
      kind: kind ?? 'internal',
      startTime: new Date(span.start).toISOString(),
    });
    return span;
  }

  finalize(status: 'success' | 'error' = 'success'): void {
    if (this.otelEnded) return;
    this.otelEnded = true;
    this.repo.finalize(this.run.id, status);
    this.otelRoot.setStatus(status === 'error' ? { code: SpanStatusCode.ERROR } : { code: SpanStatusCode.OK });
    this.otelRoot.end();
  }

  private readonly otelRoot: OtelSpan;
  private otelEnded = false;
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

function toOtelAttributes(attributes: Record<string, unknown> | undefined): Attributes {
  const result: Attributes = {};
  for (const [key, value] of Object.entries(attributes ?? {})) {
    const attribute = toOtelAttribute(value);
    if (attribute !== undefined) result[key] = attribute;
  }
  return result;
}

function toOtelAttribute(value: unknown): string | number | boolean | string[] | number[] | boolean[] | undefined {
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') return value;
  if (Array.isArray(value) && value.every((item) => typeof item === 'string')) return value;
  if (Array.isArray(value) && value.every((item) => typeof item === 'number')) return value;
  if (Array.isArray(value) && value.every((item) => typeof item === 'boolean')) return value;
  return value === undefined ? undefined : JSON.stringify(value);
}

function toOtelSpanKind(kind: 'internal' | 'llm' | 'tool' | 'retrieval' | 'agent' | undefined): SpanKind {
  switch (kind) {
    case 'llm': return SpanKind.CLIENT;
    case 'tool': return SpanKind.PRODUCER;
    default: return SpanKind.INTERNAL;
  }
}

/**
 * _db arg is unused — kept for symmetry with other utilities
 * that may want to wrap open + start. Currently the only reason
 * to import this file is to attach a Tracer to a running
 * execution; nothing else needs the bare db handle.
 */
export type _db = Database.Database;
