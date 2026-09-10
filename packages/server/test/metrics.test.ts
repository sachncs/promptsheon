import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Database from 'better-sqlite3';
import { ManifestGraphExecutor, type ExecutionTrace } from '../src/agents/executor/executor.js';
import { ManifestRepo } from '../src/repos/manifest.js';
import { SseHub } from '../src/sse/hub.js';
import { applyMigrations } from '@promptsheon/shared';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { Agent } from '@strands-agents/sdk';
import { createModel } from '../src/agents/model.js';
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

function buildManifest(): Manifest {
  const leaf = {
    id: 'm-leaf',
    version: 1,
    prompt: { systemPrompt: 'echo {{input}}', userTemplate: '{{input}}' },
    model: { provider: 'openai', modelId: 'gpt-4', temperature: 0.7, maxTokens: 100 },
    runtime: { timeoutMs: 1000, nodeTimeoutMs: 1000, totalTimeoutMs: 5000, maxRetries: 0, canaryPercent: 0, concurrencyLimit: 1 },
    context: { inputsSchema: {}, outputsSchema: {}, requiredContextVars: [] },
    memory: { enabled: false, type: 'stateless' },
    guardrails: { pre: [], post: [] },
    tools: [], mcpServers: [],
    evaluation: { datasets: [], scorers: [], passThreshold: 0.5 },
    nodes: [], edges: [], metadata: {},
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  };
  return {
    id: 'm1', version: 1,
    prompt: { systemPrompt: 'x', userTemplate: '{{input}}' },
    model: { provider: 'openai', modelId: 'gpt-4', temperature: 0.7, maxTokens: 100 },
    runtime: { timeoutMs: 1000, nodeTimeoutMs: 1000, totalTimeoutMs: 5000, maxRetries: 0, canaryPercent: 0, concurrencyLimit: 1 },
    context: { inputsSchema: {}, outputsSchema: {}, requiredContextVars: [] },
    memory: { enabled: false, type: 'stateless' },
    guardrails: { pre: [], post: [] },
    tools: [], mcpServers: [],
    evaluation: { datasets: [], scorers: [], passThreshold: 0.5 },
    nodes: [
      { id: 'a', name: 'A', description: '', goal: 'a', manifest: leaf, dependsOn: [], preGuardrails: [], postGuardrails: [], observability: { logInputs: true, logOutputs: true, trackLatency: true, trackCost: true }, hooks: { beforeInvocation: false, afterInvocation: false, beforeModelCall: false, afterModelCall: false, beforeToolCall: false, afterToolCall: false }, retry: { kind: 'exponential', maxAttempts: 3, baseDelayMs: 1000, maxDelayMs: 10000 }, conversationManager: { kind: 'sliding-window', windowSize: 20 }, state: { enabled: false, type: 'stateless' }, limits: {} },
    ],
    edges: [],
    metadata: { capabilityId: 'cap1' },
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  };
}

/**
 * Stub that records all invocations and returns scripted responses.
 */
class StubGraphExecutor extends ManifestGraphExecutor {
  public invokeCalls: Array<{ manifestHash: string; manifest: Manifest; options: unknown }> = [];
  public stubTrace: ExecutionTrace = {
    executionId: 'exec-1',
    manifestHash: 'h1',
    status: 'completed',
    startedAt: '2026-01-01T00:00:00Z',
    endedAt: '2026-01-01T00:00:00Z',
    nodeResults: {
      a: { nodeId: 'a', status: 'completed', output: 'ok', latencyMs: 100, costUsd: 0, totalTokens: 10, error: '' },
    },
    totalCost: 0, totalLatencyMs: 100, totalTokens: 10,
  };

  async execute(manifestHash: string, manifest: Manifest, options: { executionId: string; inputs: Record<string, unknown>; environment?: string; traceId?: string; signal?: AbortSignal }): Promise<ExecutionTrace> {
    this.invokeCalls.push({ manifestHash, manifest, options });
    return { ...this.stubTrace, executionId: options.executionId, manifestHash };
  }
}

describe('ManifestGraphExecutor metrics extraction', () => {
  let db: ReturnType<typeof Database>;
  let manifestRepo: ManifestRepo;
  let hub: SseHub;
  let executor: StubGraphExecutor;

  beforeEach(() => {
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
    manifestRepo = new ManifestRepo(db);
    hub = new SseHub();
    executor = new StubGraphExecutor({ config: buildConfig(), hub });
    void manifestRepo;
  });

  afterEach(() => {
    hub.stop?.();
    db.close();
  });

  it('extracted metrics shape: nodeResults has status + totals', async () => {
    const m = buildManifest();
    executor.stubTrace.nodeResults = {
      a: { nodeId: 'a', status: 'completed', output: 'ok', latencyMs: 250, costUsd: 0.0005, totalTokens: 50, error: '' },
    };
    executor.stubTrace.totalCost = 0.0005;
    executor.stubTrace.totalLatencyMs = 250;
    executor.stubTrace.totalTokens = 50;
    const trace = await executor.execute('h1', m, { executionId: 'exec-1', inputs: { foo: 'bar' } });
    expect(trace.nodeResults['a']?.status).toBe('completed');
    expect(trace.nodeResults['a']?.latencyMs).toBe(250);
    expect(trace.nodeResults['a']?.totalTokens).toBe(50);
    expect(trace.nodeResults['a']?.costUsd).toBe(0.0005);
    expect(trace.totalCost).toBe(0.0005);
    expect(trace.totalLatencyMs).toBe(250);
    expect(trace.totalTokens).toBe(50);
  });

  it('failed node has status: failed and error populated', async () => {
    const m = buildManifest();
    executor.stubTrace.nodeResults = {
      a: { nodeId: 'a', status: 'failed', output: '', latencyMs: 100, costUsd: 0, totalTokens: 0, error: 'pre-guardrail blocked' },
    };
    executor.stubTrace.status = 'failed';
    const trace = await executor.execute('h1', m, { executionId: 'exec-1', inputs: {} });
    expect(trace.status).toBe('failed');
    expect(trace.nodeResults['a']?.status).toBe('failed');
    expect(trace.nodeResults['a']?.error).toContain('guardrail');
  });

  it('executionId flows from options to trace', async () => {
    const m = buildManifest();
    const trace = await executor.execute('h1', m, { executionId: 'my-exec-id', inputs: {} });
    expect(trace.executionId).toBe('my-exec-id');
  });

  it('manifestHash flows from options to trace', async () => {
    const m = buildManifest();
    const trace = await executor.execute('specific-hash', m, { executionId: 'exec-1', inputs: {} });
    expect(trace.manifestHash).toBe('specific-hash');
  });

  it('cost is summed across nodes', async () => {
    const m = buildManifest();
    executor.stubTrace = {
      executionId: 'exec-1',
      manifestHash: 'h1',
      status: 'completed',
      startedAt: '2026-01-01T00:00:00Z',
      endedAt: '2026-01-01T00:00:00Z',
      nodeResults: {
        a: { nodeId: 'a', status: 'completed', output: '', latencyMs: 100, costUsd: 0.01, totalTokens: 100, error: '' },
        b: { nodeId: 'b', status: 'completed', output: '', latencyMs: 100, costUsd: 0.02, totalTokens: 200, error: '' },
      },
      totalCost: 0.03,
      totalLatencyMs: 200,
      totalTokens: 300,
    };
    const trace = await executor.execute('h1', m, { executionId: 'exec-1', inputs: {} });
    expect(trace.totalCost).toBe(0.03);
    expect(trace.totalTokens).toBe(300);
  });
});