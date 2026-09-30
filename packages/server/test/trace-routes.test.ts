import { describe, expect, it, vi } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import { registerTraceRoutes } from '../src/routes/trace.js';
import { TraceService, type TraceServiceDependencies } from '../src/application/trace-service.js';

function makeService(): TraceService {
  const deps: TraceServiceDependencies = {
    traces: {
      findByIdInOrg: vi.fn(() => null),
      findSpansByRun: vi.fn(() => []),
      listByOrg: vi.fn(() => ({ items: [], total: 0 })),
      rollupByOrg: vi.fn(() => []),
      operationalSummary: vi.fn(() => ({ runs: 0, errors: 0, averageLatencyMs: 0, tokens: 0, cost: 0, models: [] })),
      promptRiskByOrg: vi.fn(() => [{
        promptKey: 'org-a:agent',
        runs: 4,
        errors: 2,
        errorRate: 0.5,
        tokens: 12_000,
        cost: 0.12,
        actors: 2,
        lastSeen: '2026-09-30T00:00:00.000Z',
        signals: ['error-rate', 'token-burn'] as const,
        risk: 'high' as const,
      }]),
    },
    scores: { listByRun: vi.fn(() => []), summaryByOrg: vi.fn(() => ({ totals: 0, perEvaluator: [] })) },
    evaluator: { run: vi.fn(async () => 0) },
  };
  return new TraceService(deps);
}

describe('trace prompt-risk route', () => {
  let app: FastifyInstance;

  it('returns tenant-scoped risk signals and validates query limits', async () => {
    app = Fastify();
    app.addHook('onRequest', async (request) => {
      request.orgContext = { userId: 'admin', orgId: 'org-a', role: 'admin' };
    });
    registerTraceRoutes(app, { service: makeService(), requireAdmin: () => async () => undefined });
    await app.ready();

    const response = await app.inject({ method: 'GET', url: '/api/traces/prompt-risk?days=7&limit=10' });
    expect(response.statusCode).toBe(200);
    expect(response.json()).toMatchObject({ orgId: 'org-a', days: 7, limit: 10, items: [{ promptKey: 'org-a:agent', risk: 'high' }] });

    const invalid = await app.inject({ method: 'GET', url: '/api/traces/prompt-risk?days=0' });
    expect(invalid.statusCode).toBe(422);
    await app.close();
  });

  it('rejects the route without organization context', async () => {
    app = Fastify();
    registerTraceRoutes(app, { service: makeService(), requireAdmin: () => async () => undefined });
    await app.ready();

    const response = await app.inject({ method: 'GET', url: '/api/traces/prompt-risk' });
    expect(response.statusCode).toBe(401);
    await app.close();
  });
});
