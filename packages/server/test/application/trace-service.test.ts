import { describe, expect, it, vi } from 'vitest';
import { TraceService, type TraceServiceDependencies } from '../../src/application/trace-service.js';
import type { TraceRun } from '../../src/repos/trace.js';

const run: TraceRun = {
  id: 'trace-1',
  organizationId: 'org-a',
  actorId: null,
  executionId: 'execution-1',
  sessionId: null,
  environment: 'test',
  name: 'test trace',
  startTime: '2026-01-01T00:00:00.000Z',
  endTime: null,
  status: 'running',
  attributes: {},
  totalTokens: 0,
  totalCostUsd: 0,
  model: null,
};

function dependencies(): TraceServiceDependencies {
  return {
    traces: {
      findByIdInOrg: vi.fn((id: string, organizationId: string) =>
        id === run.id && organizationId === run.organizationId ? run : null,
      ),
      findSpansByRun: vi.fn(() => []),
      listByOrg: vi.fn(() => ({ items: [run], total: 1 })),
      rollupByOrg: vi.fn(() => []),
    },
    scores: {
      listByRun: vi.fn(() => []),
      summaryByOrg: vi.fn(() => ({ totals: 0, perEvaluator: [] })),
    },
    evaluator: { run: vi.fn(async () => 3) },
  };
}

describe('TraceService', () => {
  it('does not expose a trace across organization boundaries', () => {
    const deps = dependencies();
    const service = new TraceService(deps);

    expect(service.get('org-b', run.id)).toBeNull();
    expect(deps.traces.findSpansByRun).not.toHaveBeenCalled();
  });

  it('does not run auto-evaluation for a trace outside the organization', async () => {
    const deps = dependencies();
    const service = new TraceService(deps);

    await expect(service.autoEval('org-b', run.id, {})).resolves.toBeNull();
    expect(deps.evaluator.run).not.toHaveBeenCalled();
  });

  it('delegates authorized auto-evaluation with the requested options', async () => {
    const deps = dependencies();
    const service = new TraceService(deps);
    const options = { judgeModel: 'test-model', judgePrompt: 'score this' };

    await expect(service.autoEval('org-a', run.id, options)).resolves.toBe(3);
    expect(deps.evaluator.run).toHaveBeenCalledWith(run.id, options);
  });
});
