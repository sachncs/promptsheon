import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import { registerIdeaRoutes } from '../src/routes/idea.js';
import type { IdeaPlannerAgent } from '../src/agents/planner/index.js';
import type { AppConfig } from '@promptsheon/shared';


function buildConfig(): AppConfig {
  return {
    server: {
      port: 8080,
      host: '127.0.0.1',
      dbPath: ':memory:',
      casPath: '/tmp/cas',
      frontendPath: '/tmp/web',
      corsOrigin: '',
      logLevel: 'info',
    },
    llm: {
      provider: 'openai',
      modelId: 'gpt-4',
      apiKeyEnv: 'OPENAI_API_KEY',
      maxRetries: 3,
      timeoutMs: 30000,
    },
    auth: { enabled: false, jwtSecret: '' },
    selfEvolve: { enabled: false, defaultCooldownSec: 900, maxConcurrentCycles: 3 },
  };
}

class StubPlannerAgent {
  planCalls = 0;
  lastInput: unknown = null;
  response: unknown = { goal: 'test', nodes: [], edges: [], syntheticCases: [], passThreshold: 0.5, acceptanceCriteria: [] };

  async plan(input: unknown): Promise<unknown> {
    this.planCalls += 1;
    this.lastInput = input;
    return this.response;
  }
}

describe('POST /api/ideas/plan', () => {
  let app: FastifyInstance;
  let planner: StubPlannerAgent;

  beforeEach(async () => {
    planner = new StubPlannerAgent();
    app = Fastify();
    app.setErrorHandler((error, _request, reply) => {
      if (error.statusCode) {
        return reply.code(error.statusCode).send({ error: { code: 'APP_ERROR', message: error.message } });
      }
      return reply.code(500).send({ error: { code: 'INTERNAL_ERROR', message: error.message } });
    });
    await app.register(async (instance) => {
      await registerIdeaRoutes(instance, { planner: planner as unknown as IdeaPlannerAgent });
    });
    await app.ready();
  });

  afterEach(async () => {
    await app.close();
  });

  it('accepts a valid idea and returns the planner result', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/api/ideas/plan',
      payload: { idea: 'Build a code review bot' },
    });
    expect(response.statusCode).toBe(200);
    const body = response.json() as { goal: string };
    expect(body.goal).toBe('test');
    expect(planner.planCalls).toBe(1);
    expect((planner.lastInput as { idea: string }).idea).toBe('Build a code review bot');
  });

  it('rejects an empty idea with 422', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/api/ideas/plan',
      payload: { idea: '' },
    });
    expect(response.statusCode).toBe(422);
  });

  it('rejects a missing idea with 422', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/api/ideas/plan',
      payload: {},
    });
    expect(response.statusCode).toBe(422);
  });

  it('returns 503 when planner throws', async () => {
    const throwingPlanner = {
      plan: async () => {
        throw new Error('planner boom');
      },
    };
    const localApp = Fastify();
    localApp.setErrorHandler((error, _request, reply) => {
      if (error.statusCode) {
        return reply.code(error.statusCode).send({ error: { code: 'APP_ERROR', message: error.message } });
      }
      return reply.code(500).send({ error: { code: 'INTERNAL_ERROR', message: error.message } });
    });
    await localApp.register(async (instance) => {
      await registerIdeaRoutes(instance, { planner: throwingPlanner as unknown as IdeaPlannerAgent });
    });
    await localApp.ready();
    const response = await localApp.inject({
      method: 'POST',
      url: '/api/ideas/plan',
      payload: { idea: 'test' },
    });
    await localApp.close();
    expect(response.statusCode).toBe(503);
  });
});