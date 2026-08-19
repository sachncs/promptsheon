import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import { registerGoalObservabilityRoutes } from '../src/routes/goals.js';
import { GoalBasedEvolutionAgent } from '../src/agents/evolution/goal-evolver.js';
import { SseHub } from '../src/sse/hub.js';
import type { AppConfig } from '@promptsheon/shared';

function buildConfig(): AppConfig {
  return {
    server: { port: 8080, host: '127.0.0.1', dbPath: ':memory:', casPath: '/tmp/cas', frontendPath: '/tmp/web', corsOrigin: '', logLevel: 'info' },
    llm: { provider: 'openai', modelId: 'gpt-4', apiKeyEnv: 'OPENAI_API_KEY', maxRetries: 3, timeoutMs: 30000 },
    auth: { enabled: false, jwtSecret: '' },
    selfEvolve: { enabled: false, defaultCooldownSec: 900, maxConcurrentCycles: 3 },
  };
}

class StubAgent {
  state = new Map<string, { currentHash: string; bestHash: string; bestScore: number; iteration: number }>();
  getState(key: string) { return this.state.get(key); }
}

describe('Goal observability routes', () => {
  let app: FastifyInstance;
  let agent: StubAgent;
  let activeGoals: Array<{ manifestHash: string; bestScore: number; iterations: number; lastUpdated: string }>;

  beforeEach(async () => {
    agent = new StubAgent();
    activeGoals = [
      { manifestHash: 'h1', bestScore: 0.8, iterations: 3, lastUpdated: '2026-01-01T00:00:00Z' },
      { manifestHash: 'h2', bestScore: 0.6, iterations: 5, lastUpdated: '2026-01-02T00:00:00Z' },
    ];
    app = Fastify();
    await app.register(async (instance) => {
      await registerGoalObservabilityRoutes(instance, {
        goalEvolver: agent as unknown as GoalBasedEvolutionAgent,
        getActiveGoals: () => activeGoals,
      });
    });
    await app.ready();
  });

  afterEach(async () => {
    await app.close();
  });

  it('GET /api/goals returns active goals list', async () => {
    const response = await app.inject({ method: 'GET', url: '/api/goals' });
    expect(response.statusCode).toBe(200);
    const body = response.json() as { goals: Array<{ manifestHash: string }> };
    expect(body.goals).toHaveLength(2);
    expect(body.goals[0].manifestHash).toBe('h1');
  });

  it('GET /api/goals?limit=1 limits results', async () => {
    const response = await app.inject({ method: 'GET', url: '/api/goals?limit=1' });
    expect(response.statusCode).toBe(200);
  });

  it('GET /api/goals/:hash returns drill-down when state exists', async () => {
    agent.state.set('h1', { currentHash: 'h1', bestHash: 'h1', bestScore: 0.8, iteration: 3 });
    const response = await app.inject({ method: 'GET', url: '/api/goals/h1' });
    expect(response.statusCode).toBe(200);
    const body = response.json() as { bestScore: number; iterations: number };
    expect(body.bestScore).toBe(0.8);
    expect(body.iterations).toBe(3);
  });

  it('GET /api/goals/:hash returns 404 when state missing', async () => {
    const response = await app.inject({ method: 'GET', url: '/api/goals/missing' });
    expect(response.statusCode).toBe(404);
  });
});