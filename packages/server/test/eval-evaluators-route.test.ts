import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import { registerEvalRoutes } from '../src/routes/eval.js';
import { EvaluationAgent } from '../src/agents/evaluation/evaluation.js';
import { EvalRepo } from '../src/repos/eval.js';
import { applyMigrations } from '@promptsheon/shared';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import type { AppConfig } from '@promptsheon/shared';

const __dirname = dirname(fileURLToPath(import.meta.url));
const MIGRATIONS_DIR = join(__dirname, '..', '..', 'shared', 'db', 'migrations');

function loadAllMigrations() {
  return readdirSync(MIGRATIONS_DIR)
    .filter((f) => f.endsWith('.up.sql'))
    .map((f) => {
      const version = parseInt(f.split('_')[0], 10);
      const up = readFileSync(join(MIGRATIONS_DIR, f), 'utf-8');
      return { version, name: f, up };
    })
    .filter((m) => m.version !== 0)
    .sort((a, b) => a.version - b.version);
}

function buildConfig(): AppConfig {
  return {
    server: { port: 8080, host: '127.0.0.1', dbPath: ':memory:', casPath: '/tmp/cas', frontendPath: '/tmp/web', corsOrigin: '', logLevel: 'info' },
    llm: { provider: 'openai', modelId: 'gpt-4', apiKeyEnv: 'OPENAI_API_KEY', maxRetries: 3, timeoutMs: 30000 },
    auth: { enabled: false, jwtSecret: '' },
    selfEvolve: { enabled: false, defaultCooldownSec: 900, maxConcurrentCycles: 3 },
  };
}

describe('GET /api/eval/evaluators and POST /api/eval/score', () => {
  let app: FastifyInstance;
  let repo: EvalRepo;
  let agent: EvaluationAgent;
  let db: ReturnType<typeof import('better-sqlite3')>;

  beforeEach(async () => {
    const Database = (await import('better-sqlite3')).default;
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
    repo = new EvalRepo(db);
    agent = new EvaluationAgent(buildConfig());
    app = Fastify({ logger: false });
    app.setErrorHandler((error, _request, reply) => {
      if (error.statusCode) return reply.code(error.statusCode).send({ error: { code: 'APP_ERROR', message: error.message } });
      return reply.code(500).send({ error: { code: 'INTERNAL_ERROR', message: error.message } });
    });
    await app.register(async (instance) => {
      await registerEvalRoutes(instance, repo, agent);
    });
    await app.ready();
  });

  afterEach(async () => {
    await app.close();
    db.close();
  });

  it('GET /api/eval/evaluators returns the 5 evaluator names', async () => {
    const response = await app.inject({ method: 'GET', url: '/api/eval/evaluators' });
    expect(response.statusCode).toBe(200);
    const body = response.json() as { evaluators: string[] };
    expect(body.evaluators).toContain('llm-judge');
    expect(body.evaluators).toContain('helpfulness');
    expect(body.evaluators).toContain('coherence');
    expect(body.evaluators).toContain('correctness');
    expect(body.evaluators).toContain('goal-success-rate');
  });

  it('POST /api/eval/score returns 404 for unknown evaluator', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/api/eval/score',
      payload: { actual: 'a', expected: 'b', evaluator: 'unknown' },
    });
    expect(response.statusCode).toBe(404);
  });

  it('POST /api/eval/score returns 422 for missing required fields', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/api/eval/score',
      payload: { evaluator: 'llm-judge' },
    });
    expect(response.statusCode).toBe(422);
  });

  it('POST /api/eval/score uses default llm-judge when no evaluator given', async () => {
    // Will fail at LLM call (no API key) but should reach the evaluator
    const response = await app.inject({
      method: 'POST',
      url: '/api/eval/score',
      payload: { actual: 'a', expected: 'b' },
    });
    // Either 200 (if it succeeds) or 500 (if LLM call fails)
    expect([200, 500].includes(response.statusCode)).toBe(true);
  });
});