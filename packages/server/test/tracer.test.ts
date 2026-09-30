import { describe, expect, it, vi } from 'vitest';
import type { Span as OtelSpan, Tracer as OtelTracer } from '@opentelemetry/api';
import type { TraceRepo, TraceRun, TraceSpan } from '../src/repos/trace.js';
import { Tracer } from '../src/observability/tracer.js';

describe('Tracer OpenTelemetry bridge', () => {
  it('mirrors root and child lifecycle into the configured OTel tracer', () => {
    const rootSpan = fakeOtelSpan();
    const childSpan = fakeOtelSpan();
    const otelTracer = {
      startSpan: vi.fn()
        .mockReturnValueOnce(rootSpan)
        .mockReturnValueOnce(childSpan),
    } as unknown as OtelTracer;
    const persistedSpan = { id: 'span-1' } as TraceSpan;
    const repo = {
      addSpan: vi.fn(),
      finishSpan: vi.fn(),
      findSpansByRun: vi.fn(() => [persistedSpan]),
      finalize: vi.fn(),
    } as unknown as TraceRepo;
    const run: TraceRun = {
      id: 'run-1', organizationId: 'org-1', actorId: null, executionId: 'exec-1',
      sessionId: null, environment: 'test', name: 'execution:test', startTime: '2026-01-01T00:00:00.000Z',
      endTime: null, status: 'running', attributes: { workspaceId: 'workspace-1' },
      totalTokens: 0, totalCostUsd: 0, model: null,
    };

    const tracer = new Tracer(run, repo, otelTracer);
    const span = tracer.span('model.call', 'llm');
    span.setAttribute('attempt', 1);
    span.setStatus('error');
    span.end();
    tracer.finalize('error');

    expect(otelTracer.startSpan).toHaveBeenCalledTimes(2);
    expect(otelTracer.startSpan).toHaveBeenNthCalledWith(1, 'execution:test', expect.objectContaining({ attributes: { workspaceId: 'workspace-1' } }));
    expect(otelTracer.startSpan).toHaveBeenNthCalledWith(2, 'model.call', expect.objectContaining({ kind: 2 }), expect.anything());
    expect(childSpan.setAttribute).toHaveBeenCalledWith('attempt', 1);
    expect(childSpan.setStatus).toHaveBeenCalledWith({ code: 2 });
    expect(childSpan.end).toHaveBeenCalledOnce();
    expect(rootSpan.setStatus).toHaveBeenCalledWith({ code: 2 });
    expect(rootSpan.end).toHaveBeenCalledOnce();
  });
});

function fakeOtelSpan(): OtelSpan {
  return {
    setStatus: vi.fn(),
    setAttribute: vi.fn(),
    end: vi.fn(),
  } as unknown as OtelSpan;
}
