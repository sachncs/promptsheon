import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import { registerGoalEvolveRoutes } from '../src/routes/goal-evolve.js';
import { GoalBasedEvolutionAgent } from '../src/agents/evolution/goal-evolver.js';
import { ManifestRepo } from '../src/repos/manifest.js';
import type { AppConfig, EvolutionResult } from '../src/agents/evolution/goal-evolver.js';
import { applyMigrations } from '@promptsheon/shared';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

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

class StubGoalEvolver {
  calls = 0;
  lastInput: unknown = null;
  response: EvolutionResult = {
    passed: true,
    manifestHash: 'h1',
    bestScore: 0.9,
    bestManifestHash: 'h1',
    iterations: 1,
    totalCost: 0.01,
    history: [],
  };
  error: Error | null = null;

  async evolve(manifestHash: string, _manifest: unknown, options: unknown): Promise<EvolutionResult> {
    this.calls += 1;
    this.lastInput = { manifestHash, options };
    if (this.error) throw this.error;
    return this.response;
  }
}

describe('POST /api/manifests/:hash/evolve', () => {
  let app: FastifyInstance;
  let evolver: StubGoalEvolver;
  let manifestRepo: ManifestRepo;
  let db: ReturnType<typeof import('better-sqlite3')>;

  beforeEach(async () => {
    const Database = (await import('better-sqlite3')).default;
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
    db.prepare(`
      INSERT INTO workspaces (id, name, organization, created_at, updated_at)
      VALUES ('ws1', 'Test WS', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
    `).run();
    db.prepare(`
      INSERT INTO projects (id, workspace_id, name, description, created_at, updated_at)
      VALUES ('proj1', 'ws1', 'Test Project', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
    `).run();
    db.prepare(`
      INSERT INTO capabilities (id, project_id, name, description, created_at, updated_at)
      VALUES ('cap1', 'proj1', 'Test', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
    `).run();

    manifestRepo = new ManifestRepo(db);
    evolver = new StubGoalEvolver();
    app = Fastify();
    app.setErrorHandler((error, _request, reply) => {
      if (error.name === 'NotFoundError') {
        return reply.code(404).send({ error: { code: 'NOT_FOUND', message: error.message } });
      }
      if (error.statusCode) {
        return reply.code(error.statusCode).send({ error: { code: 'APP_ERROR', message: error.message } });
      }
      return reply.code(500).send({ error: { code: 'INTERNAL_ERROR', message: error.message } });
    });
    await app.register(async (instance) => {
      await registerGoalEvolveRoutes(instance, {
        goalEvolver: evolver as unknown as GoalBasedEvolutionAgent,
        manifestRepo,
      });
    });
    await app.ready();
  });

  afterEach(async () => {
    await app.close();
    db.close();
  });

  it('returns 422 when body is missing', async () => {
    const response = await app.inject({ method: 'POST', url: '/api/manifests/h/evolve' });
    expect(response.statusCode).toBe(422);
  });

  it('returns 404 when manifest hash is unknown', async () => {
    const response = await app.inject({ method: 'POST', url: '/api/manifests/unknown/evolve', payload: {} });
    expect(response.statusCode).toBe(404);
  });

  it('returns 503 when evolver throws', async () => {
    await manifestRepo.create(
      {
        id: 'm1',
        version: 1,
        prompt: { systemPrompt: 'x', userTemplate: '' },
        model: { provider: 'openai', modelId: 'gpt-4', temperature: 0.7, maxTokens: 100 },
        runtime: { timeoutMs: 1000, nodeTimeoutMs: 1000, totalTimeoutMs: 5000, maxRetries: 0, canaryPercent: 0, concurrencyLimit: 1 },
        context: { inputsSchema: {}, outputsSchema: {}, requiredContextVars: [] },
        memory: { enabled: false, type: 'stateless' },
        guardrails: { pre: [], post: [] },
        tools: [], mcpServers: [],
        evaluation: { datasets: [], scorers: [], passThreshold: 0.5 },
        nodes: [], edges: [], metadata: { capabilityId: 'cap1' },
        createdAt: '2026-01-01T00:00:00Z',
        updatedAt: '2026-01-01T00:00:00Z',
      },
      { goal: 'g', createdBy: 'tester' },
    );
    const hash = (await import('../src/repos/manifest.js')).computeManifestHash({
      id: 'm1',
      version: 1,
      prompt: { systemPrompt: 'x', userTemplate: '' },
      model: { provider: 'openai', modelId: 'gpt-4', temperature: 0.7, maxTokens: 100 },
      runtime: { timeoutMs: 1000, nodeTimeoutMs: 1000, totalTimeoutMs: 5000, maxRetries: 0, canaryPercent: 0, concurrencyLimit: 1 },
      context: { inputsSchema: {}, outputsSchema: {}, requiredContextVars: [] },
      memory: { enabled: false, type: 'stateless' },
      guardrails: { pre: [], post: [] },
      tools: [], mcpServers: [],
      evaluation: { datasets: [], scorers: [], passThreshold: 0.5 },
      nodes: [], edges: [], metadata: { capabilityId: 'cap1' },
      createdAt: '2026-01-01T00:00:00Z',
      updatedAt: '2026-01-01T00:00:00Z',
    });
    evolver.error = new Error('boom');
    const response = await app.inject({ method: 'POST', url: `/api/manifests/${hash}/evolve`, payload: {} });
    expect(response.statusCode).toBe(503);
  });
});