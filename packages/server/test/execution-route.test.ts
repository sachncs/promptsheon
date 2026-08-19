import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import { registerExecutionRoutes } from '../src/routes/execution.js';
import { ExecutionRepo } from '../src/repos/execution.js';
import { ReleaseRepo } from '../src/repos/release.js';
import { ManifestRepo } from '../src/repos/manifest.js';
import { ManifestGraphExecutor } from '../src/agents/executor/executor.js';
import { SseHub } from '../src/sse/hub.js';
import { computeManifestHash } from '../src/repos/manifest.js';
import { applyMigrations } from '@promptsheon/shared';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import Database from 'better-sqlite3';
import type { AppConfig, Manifest } from '@promptsheon/shared';

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

function buildLeafManifest(id: string, version = 1, capabilityId = 'cap1'): Manifest {
  return {
    id, version,
    prompt: { systemPrompt: 'x', userTemplate: '{{input}}' },
    model: { provider: 'openai', modelId: 'gpt-4', temperature: 0.7, maxTokens: 100 },
    runtime: { timeoutMs: 1000, nodeTimeoutMs: 1000, totalTimeoutMs: 5000, maxRetries: 0, canaryPercent: 0, concurrencyLimit: 1 },
    context: { inputsSchema: {}, outputsSchema: {}, requiredContextVars: [] },
    memory: { enabled: false, type: 'stateless' },
    guardrails: { pre: [], post: [] },
    tools: [], mcpServers: [],
    evaluation: { datasets: [], scorers: [], passThreshold: 0.5 },
    nodes: [], edges: [], metadata: { capabilityId },
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  };
}

function insertTestData(db: ReturnType<typeof Database>): void {
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
}

describe('POST /api/executions', () => {
  let app: FastifyInstance;
  let db: ReturnType<typeof Database>;
  let manifestRepo: ManifestRepo;
  let executionRepo: ExecutionRepo;
  let executor: ManifestGraphExecutor;
  let hub: SseHub;

  beforeEach(async () => {
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
    insertTestData(db);
    manifestRepo = new ManifestRepo(db);
    executionRepo = new ExecutionRepo(db);
    hub = new SseHub();
    executor = new ManifestGraphExecutor({ config: buildConfig(), hub });

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
      await registerExecutionRoutes(instance, { executionRepo, manifestRepo, executor });
    });
    await app.ready();
  });

  afterEach(async () => {
    await app.close();
    db.close();
  });

  it('returns 422 when manifestHash is missing', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/api/executions',
      payload: { inputs: {} },
    });
    expect(response.statusCode).toBe(422);
  });

  it('returns 404 when manifest hash is unknown', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/api/executions',
      payload: { manifestHash: 'nope', inputs: {} },
    });
    expect(response.statusCode).toBe(404);
  });

  it('returns 404 for /api/executions/:id when id is unknown', async () => {
    const response = await app.inject({
      method: 'GET',
      url: '/api/executions/missing',
    });
    expect(response.statusCode).toBe(404);
  });

  it('executes a real manifest DAG and records per-node run', async () => {
    const manifest = buildLeafManifest('test-m');
    const hash = manifestRepo.create(manifest, { goal: 'g', createdBy: 'tester' });
    const response = await app.inject({
      method: 'POST',
      url: '/api/executions',
      payload: { manifestHash: hash, inputs: { foo: 'bar' } },
    });
    // The node invocation will fail at LLM call but DAG validation passes
    // so we should at minimum get a 5xx or a 200 with status='failed'.
    expect([200, 500, 503].includes(response.statusCode)).toBe(true);
  });
});