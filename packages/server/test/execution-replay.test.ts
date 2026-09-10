import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import Database from 'better-sqlite3';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { applyMigrations, type AppConfig, type Execution, type Manifest } from '@promptsheon/shared';
import { ExecutionRepo, ReplayInputsUnavailableError } from '../src/repos/execution.js';
import { ManifestRepo } from '../src/repos/manifest.js';
import { TraceRepo } from '../src/repos/trace.js';
import { registerExecutionRoutes } from '../src/routes/execution.js';
import { ManifestGraphExecutor } from '../src/agents/executor/executor.js';
import { SseHub } from '../src/sse/hub.js';
import type { ExecutionTrace } from '../src/agents/executor/index.js';

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
    server: {
      port: 8080,
      host: '127.0.0.1',
      dbPath: ':memory:',
      casPath: '/tmp/cas',
      frontendPath: '/tmp/web',
      corsOrigin: '',
      logLevel: 'info',
      nodeEnv: 'test',
      fipsMode: false,
    },
    llm: {
      defaultProvider: 'openai',
      defaultModel: 'gpt-4',
      apiKeyEnvVar: 'OPENAI_API_KEY',
      maxRetries: 0,
      timeoutMs: 1000,
    },
    auth: { enabled: false, jwtSecret: '' },
    selfEvolve: { enabled: false, defaultCooldownSec: 900, maxConcurrent: 1 },
  };
}

function buildLeafManifest(id = 'm1'): Manifest {
  return {
    id,
    version: 1,
    prompt: { systemPrompt: 'x', userTemplate: '{{input}}' },
    model: { provider: 'openai', modelId: 'gpt-4', temperature: 0.7, maxTokens: 100 },
    runtime: {
      timeoutMs: 1000,
      nodeTimeoutMs: 1000,
      totalTimeoutMs: 5000,
      maxRetries: 0,
      canaryPercent: 0,
      concurrencyLimit: 1,
    },
    context: { inputsSchema: {}, outputsSchema: {}, requiredContextVars: [] },
    memory: { enabled: false, type: 'stateless' },
    guardrails: { pre: [], post: [] },
    tools: [],
    mcpServers: [],
    evaluation: { datasets: [], scorers: [], passThreshold: 0.5 },
    nodes: [],
    edges: [],
    metadata: { capabilityId: 'cap1' },
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  };
}

function insertTestData(db: ReturnType<typeof Database>): void {
  db.prepare(
    `INSERT INTO workspaces (id, name, organization, created_at, updated_at)
     VALUES ('ws1', 'Test WS', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
  ).run();
  db.prepare(
    `INSERT INTO projects (id, workspace_id, name, description, created_at, updated_at)
     VALUES ('proj1', 'ws1', 'Test Project', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
  ).run();
  db.prepare(
    `INSERT INTO capabilities (id, project_id, name, description, created_at, updated_at)
     VALUES ('cap1', 'proj1', 'Test', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
  ).run();
  db.prepare(
    `INSERT INTO capability_versions (id, capability_id, version, manifest, manifest_hash, created_at, created_by)
     VALUES ('cv1', 'cap1', 1, '{}', 'placeholder', '2026-01-01T00:00:00Z', 'tester')`,
  ).run();
}

interface TestHarness {
  db: ReturnType<typeof Database>;
  app: FastifyInstance;
  executionRepo: ExecutionRepo;
  manifestRepo: ManifestRepo;
  traceRepo: TraceRepo;
  executor: ManifestGraphExecutor;
  hub: SseHub;
}

async function setupHarness(): Promise<TestHarness> {
  const db = new Database(':memory:');
  db.pragma('foreign_keys = ON');
  applyMigrations(db, loadAllMigrations());
  insertTestData(db);
  const manifestRepo = new ManifestRepo(db);
  const executionRepo = new ExecutionRepo(db);
  const traceRepo = new TraceRepo(db);
  const hub = new SseHub();
  const executor = new ManifestGraphExecutor({ config: buildConfig(), hub });
  const app = Fastify();
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
    await registerExecutionRoutes(instance, {
      executionRepo,
      releaseRepo: { findActiveByManifestHash: () => [] } as never,
      manifestRepo,
      versionRepo: { findById: () => null } as never,
      traceRepo,
      executor,
    });
  });
  await app.ready();
  return { db, app, executionRepo, manifestRepo, traceRepo, executor, hub };
}

describe('ExecutionReplayService / POST /api/executions/:id/replay', () => {
  let h: TestHarness;

  beforeEach(async () => {
    h = await setupHarness();
  });
  afterEach(async () => {
    await h.app.close();
    h.db.close();
  });

  it('finds replay context for a fresh execution and parses inputs', () => {
    const original = h.executionRepo.create({
      capabilityVersionId: 'cv1',
      inputs: JSON.stringify({ foo: 'bar', n: 42 }),
      inputHash: 'abc123',
      outputs: JSON.stringify({ node1: 'original output' }),
      model: 'gpt-4',
      provider: 'openai',
      latencyMs: 100,
      costUsd: 0.001,
      promptTokens: 10,
      completionTokens: 5,
      totalTokens: 15,
      error: '',
      traceId: 'trace-1',
      environment: 'prod',
    });
    const ctx = h.executionRepo.findReplayContext(original.id);
    expect(ctx).not.toBeNull();
    expect(ctx!.manifestHash).toBe('placeholder');
    expect(ctx!.parsedInputs).toEqual({ foo: 'bar', n: 42 });
    expect(ctx!.execution.id).toBe(original.id);
  });

  it('throws ReplayInputsUnavailableError for legacy hash-stored inputs', () => {
    const id = 'legacy-row';
    h.db
      .prepare(
        `INSERT INTO executions (id, capability_version_id, inputs, outputs, model, provider, latency_ms, cost_usd, prompt_tokens, completion_tokens, total_tokens, error, trace_id, environment, timestamp)
         VALUES (?, 'cv1', ?, '{}', '', '', 0, 0, 0, 0, 0, '', '', '', '2026-01-01T00:00:00Z')`,
      )
      .run(id, 'not-json-pre-migr');
    expect(() => h.executionRepo.findReplayContext(id)).toThrow(ReplayInputsUnavailableError);
  });

  it('incrementReplayCount is idempotent and updates the count', () => {
    const original = h.executionRepo.create({
      capabilityVersionId: 'cv1',
      inputs: JSON.stringify({}),
      outputs: '{}',
      model: '',
      provider: '',
      latencyMs: 0,
      costUsd: 0,
      promptTokens: 0,
      completionTokens: 0,
      totalTokens: 0,
      error: '',
      traceId: '',
      environment: '',
    });
    expect(h.executionRepo.incrementReplayCount(original.id)).toBe(1);
    expect(h.executionRepo.incrementReplayCount(original.id)).toBe(2);
    expect(h.executionRepo.incrementReplayCount(original.id)).toBe(3);
    const row = h.executionRepo.findById(original.id);
    expect(row?.replayCount).toBe(3);
  });

  it('recordReplay persists and findReplaysByOriginal returns them all', () => {
    const original = h.executionRepo.create({
      capabilityVersionId: 'cv1',
      inputs: '{}',
      outputs: '{}',
      model: '',
      provider: '',
      latencyMs: 0,
      costUsd: 0,
      promptTokens: 0,
      completionTokens: 0,
      totalTokens: 0,
      error: '',
      traceId: '',
      environment: '',
    });
    const first = h.executionRepo.recordReplay({
      originalExecutionId: original.id,
      replayExecutionId: null,
      outcome: 'failed',
      inputsMatch: true,
      manifestMatch: false,
      modelMatch: true,
      environmentMatch: true,
      diffSummary: JSON.stringify({ reason: 'manifest_missing' }),
    });
    const replayExecutionId = h.executionRepo.create({
      capabilityVersionId: 'cv1',
      inputs: JSON.stringify({}),
      outputs: '{}',
      model: '',
      provider: '',
      latencyMs: 0,
      costUsd: 0,
      promptTokens: 0,
      completionTokens: 0,
      totalTokens: 0,
      error: '',
      traceId: '',
      environment: '',
      replayOf: original.id,
    }).id;
    const second = h.executionRepo.recordReplay({
      originalExecutionId: original.id,
      replayExecutionId,
      outcome: 'completed',
      inputsMatch: true,
      manifestMatch: true,
      modelMatch: true,
      environmentMatch: true,
      diffSummary: null,
    });
    const replays = h.executionRepo.findReplaysByOriginal(original.id);
    expect(replays).toHaveLength(2);
    const ids = replays.map((r) => r.id).sort();
    expect(ids).toEqual([first.id, second.id].sort());
    const byId = new Map(replays.map((r) => [r.id, r]));
    expect(byId.get(first.id)!.outcome).toBe('failed');
    expect(byId.get(second.id)!.outcome).toBe('completed');
    expect(byId.get(second.id)!.manifestMatch).toBe(true);
    expect(byId.get(first.id)!.manifestMatch).toBe(false);
  });

  it('POST /api/executions/:id/replay returns 404 for unknown execution', async () => {
    const response = await h.app.inject({
      method: 'POST',
      url: '/api/executions/unknown-id/replay',
    });
    expect(response.statusCode).toBe(404);
    expect(response.json()).toMatchObject({
      error: { code: 'EXECUTION_NOT_FOUND' },
    });
  });

  it('POST /api/executions/:id/replay returns 409 for legacy hash-stored inputs', async () => {
    const id = 'legacy';
    h.db
      .prepare(
        `INSERT INTO executions (id, capability_version_id, inputs, outputs, model, provider, latency_ms, cost_usd, prompt_tokens, completion_tokens, total_tokens, error, trace_id, environment, timestamp)
         VALUES (?, 'cv1', 'deadbeef', '{}', '', '', 0, 0, 0, 0, 0, '', '', '', '2026-01-01T00:00:00Z')`,
      )
      .run(id);
    const response = await h.app.inject({
      method: 'POST',
      url: `/api/executions/${id}/replay`,
    });
    expect(response.statusCode).toBe(409);
    expect(response.json()).toMatchObject({
      error: { code: 'REPLAY_INPUTS_UNAVAILABLE' },
    });
  });

  it('GET /api/executions/:id/replays returns 404 for unknown execution', async () => {
    const response = await h.app.inject({
      method: 'GET',
      url: '/api/executions/unknown/replays',
    });
    expect(response.statusCode).toBe(404);
  });

  it('GET /api/executions/:id/replays returns [] for an execution with no replays', async () => {
    const original = h.executionRepo.create({
      capabilityVersionId: 'cv1',
      inputs: JSON.stringify({ a: 1 }),
      outputs: '{}',
      model: '',
      provider: '',
      latencyMs: 0,
      costUsd: 0,
      promptTokens: 0,
      completionTokens: 0,
      totalTokens: 0,
      error: '',
      traceId: '',
      environment: '',
    });
    const response = await h.app.inject({
      method: 'GET',
      url: `/api/executions/${original.id}/replays`,
    });
    expect(response.statusCode).toBe(200);
    expect(response.json()).toEqual({ items: [] });
  });

  it('round-trip: replay succeeds against a stubbed executor and links via replay_of', async () => {
    // Replace the executor's real execute with a stub. The harness
    // exposes the executor instance; we monkey-patch its method.
    const original = h.executionRepo.create({
      capabilityVersionId: 'cv1',
      inputs: JSON.stringify({ q: 'hello' }),
      inputHash: 'h1',
      outputs: JSON.stringify({ node1: 'first' }),
      model: 'gpt-4',
      provider: 'openai',
      latencyMs: 100,
      costUsd: 0.001,
      promptTokens: 10,
      completionTokens: 5,
      totalTokens: 15,
      error: '',
      traceId: 'trace-1',
      environment: 'prod',
    });
    // Insert a real manifest so replay can resolve it
    const manifest = buildLeafManifest('m-replay');
    const manifestHash = h.manifestRepo.create(manifest, { goal: 'g', createdBy: 'tester' });
    h.db.prepare(`UPDATE capability_versions SET manifest_hash = ? WHERE id = 'cv1'`).run(manifestHash);

    const stubTrace: ExecutionTrace = {
      executionId: 'will-be-overwritten',
      manifestHash,
      status: 'completed',
      startedAt: '2026-01-01T00:00:00Z',
      endedAt: '2026-01-01T00:00:01Z',
      nodeResults: { node1: { nodeId: 'node1', status: 'completed', output: 'replayed output', latencyMs: 50, costUsd: 0.001, totalTokens: 12, error: '' } },
      totalCost: 0.001,
      totalLatencyMs: 50,
      totalTokens: 12,
    };
    const stub = async (
      _hash: string,
      _m: Manifest,
      options: { executionId: string; traceRunId?: string; inputs?: Record<string, unknown> },
    ): Promise<ExecutionTrace> => ({
      ...stubTrace,
      executionId: options.executionId,
    });
    (h.executor as unknown as { execute: typeof stub }).execute = stub;

    const response = await h.app.inject({
      method: 'POST',
      url: `/api/executions/${original.id}/replay`,
    });
    if (response.statusCode !== 201) {
      throw new Error(`replay returned ${response.statusCode}: ${response.body}`);
    }
    expect(response.statusCode).toBe(201);
    const body = response.json() as {
      replayExecutionId: string;
      replayOf: string;
      outcome: string;
      original: Execution;
      replayed: Execution;
      diff: { addedNodes: string[]; removedNodes: string[]; changedNodes: unknown[] };
    };
    expect(body.replayOf).toBe(original.id);
    expect(body.replayed.replayOf).toBe(original.id);
    expect(body.replayed.environment).toBe('prod');
    expect(body.replayed.model).toBe('gpt-4');
    expect(body.outcome).toBe('diverged');
    expect(body.diff.changedNodes.length).toBe(1);

    // Original's replay_count was incremented
    const fresh = h.executionRepo.findById(original.id);
    expect(fresh?.replayCount).toBe(1);

    // Replay lineage log has one row
    const replays = h.executionRepo.findReplaysByOriginal(original.id);
    expect(replays).toHaveLength(1);
    expect(replays[0]!.outcome).toBe('diverged');
    expect(replays[0]!.manifestMatch).toBe(true);
    expect(replays[0]!.environmentMatch).toBe(true);
    expect(replays[0]!.inputsMatch).toBe(true);
  });
});