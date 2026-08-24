import { describe, it, expect, beforeEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import { registerSelfEvolveRoutes } from '../src/routes/self-evolve.js';
import type { CapabilityRepo } from '../src/repos/capability.js';
import type { EvalRepo } from '../src/repos/eval.js';
import type { Capability } from '@promptsheon/shared';

class StubRepo {
  private readonly items = new Map<string, Capability>();
  put(c: Capability) { this.items.set(c.id, c); }
  findById(id: string): Capability | null { return this.items.get(id) ?? null; }
}

class StubEvalRepo {}

class StubEvolutionAgent {
  private readonly states = new Map<string, { status: string; cycleCount: number }>();
  setState(id: string, state: { status: string; cycleCount: number }) {
    this.states.set(id, state);
  }
  getState(id: string): { status: string; cycleCount: number } | undefined {
    return this.states.get(id);
  }
  async runCycle(capabilityId: string) {
    return { status: 'completed', capabilityId, ranAt: new Date().toISOString() };
  }
}

const CAP_ID = '00000000-0000-4000-8000-0000000000aa';

describe('self-evolve path reconciliation', () => {
  let app: FastifyInstance;
  let capRepo: StubRepo & CapabilityRepo;
  let evolutionAgent: StubEvolutionAgent;

  beforeEach(async () => {
    capRepo = new StubRepo() as unknown as StubRepo & CapabilityRepo;
    capRepo.put({
      id: CAP_ID,
      projectId: 'p1',
      name: 'C',
      description: '',
      createdAt: '',
      updatedAt: '',
      selfEvolveEnabled: false,
      selfEvolveMinScore: 0,
      selfEvolveMaxRevisions: 0,
      selfEvolveCooldownSec: 0,
      selfEvolveTargetEnv: '',
      selfEvolveDatasetId: null,
    });
    evolutionAgent = new StubEvolutionAgent();
    evolutionAgent.setState(CAP_ID, { status: 'idle', cycleCount: 0 });
    app = Fastify({ logger: false });
    registerSelfEvolveRoutes(
      app,
      evolutionAgent as unknown as Parameters<typeof registerSelfEvolveRoutes>[1],
      capRepo,
      new StubEvalRepo() as unknown as EvalRepo,
    );
    await app.ready();
  });

  it('GET /api/self-evolve/:id/state returns state', async () => {
    const r = await app.inject({ method: 'GET', url: `/api/self-evolve/${CAP_ID}/state` });
    expect(r.statusCode).toBe(200);
    const body = r.json() as { status: string; cycleCount: number };
    expect(body.status).toBe('idle');
    expect(body.cycleCount).toBe(0);
  });

  it('GET /api/capabilities/:id/self-evolve returns the same state', async () => {
    const r = await app.inject({ method: 'GET', url: `/api/capabilities/${CAP_ID}/self-evolve` });
    expect(r.statusCode).toBe(200);
    const body = r.json() as { status: string };
    expect(body.status).toBe('idle');
  });

  it('GET /api/capabilities/:id/self-evolve returns idle stub when nothing registered', async () => {
    const r = await app.inject({
      method: 'GET',
      url: '/api/capabilities/00000000-0000-4000-8000-deadbeef/self-evolve',
    });
    expect(r.statusCode).toBe(200);
    const body = r.json() as { status: string; cycleCount: number };
    expect(body.status).toBe('idle');
    expect(body.cycleCount).toBe(0);
  });

  it('POST /api/self-evolve/run with body runs a cycle', async () => {
    const r = await app.inject({
      method: 'POST',
      url: '/api/self-evolve/run',
      payload: { capabilityId: CAP_ID },
    });
    expect(r.statusCode).toBe(200);
  });

  it('POST /api/capabilities/:id/self-evolve/run runs a cycle without body', async () => {
    const r = await app.inject({
      method: 'POST',
      url: `/api/capabilities/${CAP_ID}/self-evolve/run`,
      payload: {},
    });
    expect(r.statusCode).toBe(200);
  });

  it('POST /api/capabilities/:unknown/self-evolve/run returns 404', async () => {
    const r = await app.inject({
      method: 'POST',
      url: '/api/capabilities/00000000-0000-4000-8000-deadbeef/self-evolve/run',
      payload: {},
    });
    expect(r.statusCode).toBe(404);
  });
});
