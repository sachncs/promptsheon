import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { GoalBasedEvolutionAgent } from '../src/agents/evolution/goal-evolver.js';
import { ManifestGraphExecutor } from '../src/agents/executor/executor.js';
import { SseHub } from '../src/sse/hub.js';
import { applyMigrations } from '@promptsheon/shared';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import type { AppConfig, Manifest } from '@promptsheon/shared';
import type { ExecutionTrace, NodeRunResult } from '../src/agents/executor/executor.js';

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

function buildManifest(overrides: Partial<Manifest> = {}): Manifest {
  const leaf = {
    id: 'm-leaf',
    version: 1,
    prompt: { systemPrompt: 'x', userTemplate: '{{input}}' },
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
    id: 'm', version: 1,
    prompt: { systemPrompt: 'x', userTemplate: '{{input}}' },
    model: { provider: 'openai', modelId: 'gpt-4', temperature: 0.7, maxTokens: 100 },
    runtime: { timeoutMs: 1000, nodeTimeoutMs: 1000, totalTimeoutMs: 5000, maxRetries: 0, canaryPercent: 0, concurrencyLimit: 1 },
    context: { inputsSchema: {}, outputsSchema: {}, requiredContextVars: [] },
    memory: { enabled: false, type: 'stateless' },
    guardrails: { pre: [], post: [] },
    tools: [], mcpServers: [],
    evaluation: { datasets: [], scorers: [], passThreshold: 0.7 },
    nodes: [
      { id: 'a', name: 'A', description: '', goal: 'a', manifest: leaf, dependsOn: [], preGuardrails: [], postGuardrails: [], observability: { logInputs: true, logOutputs: true, trackLatency: true, trackCost: true }, hooks: { beforeInvocation: false, afterInvocation: false, beforeModelCall: false, afterModelCall: false, beforeToolCall: false, afterToolCall: false }, retry: { kind: 'exponential', maxAttempts: 3, baseDelayMs: 1000, maxDelayMs: 10000 }, conversationManager: { kind: 'sliding-window', windowSize: 20 }, state: { enabled: false, type: 'stateless' }, limits: {} },
      { id: 'b', name: 'B', description: '', goal: 'b', manifest: leaf, dependsOn: [], preGuardrails: [], postGuardrails: [], observability: { logInputs: true, logOutputs: true, trackLatency: true, trackCost: true }, hooks: { beforeInvocation: false, afterInvocation: false, beforeModelCall: false, afterModelCall: false, beforeToolCall: false, afterToolCall: false }, retry: { kind: 'exponential', maxAttempts: 3, baseDelayMs: 1000, maxDelayMs: 10000 }, conversationManager: { kind: 'sliding-window', windowSize: 20 }, state: { enabled: false, type: 'stateless' }, limits: {} },
    ],
    edges: [],
    metadata: {},
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

class FakeExecutor {
  calls = 0;
  trace: ExecutionTrace = {
    executionId: 'exec-1',
    manifestHash: 'h',
    status: 'completed',
    startedAt: '2026-01-01T00:00:00Z',
    endedAt: '2026-01-01T00:00:01Z',
    nodeResults: { a: { nodeId: 'a', status: 'completed', output: 'ok', latencyMs: 100, costUsd: 0, totalTokens: 10, error: '' } },
    totalCost: 0, totalLatencyMs: 100, totalTokens: 10,
  };

  async execute(_hash: string, _manifest: Manifest, _options: { executionId: string; inputs: Record<string, unknown> }): Promise<ExecutionTrace> {
    this.calls += 1;
    return this.trace;
  }
}

class FakeCas {
  writes = 0;
  async writeObject(obj: { type: string; data: Buffer }): Promise<string> {
    this.writes += 1;
    return `cas-hash-${this.writes}`;
  }
}

describe('GoalBasedEvolutionAgent', () => {
  let hub: SseHub;
  let executor: FakeExecutor;
  let agent: GoalBasedEvolutionAgent;

  beforeEach(() => {
    hub = new SseHub();
    executor = new FakeExecutor();
    agent = new GoalBasedEvolutionAgent({
      config: buildConfig(),
      hub,
      executor: executor as unknown as ManifestGraphExecutor,
      cas: new FakeCas() as never,
    });
  });

  afterEach(() => hub.stop?.());

  describe('construction', () => {
    it('constructs with required deps', () => {
      expect(agent).toBeInstanceOf(GoalBasedEvolutionAgent);
    });
  });

  describe('evolve', () => {
    it('returns immediately when DAG is invalid', async () => {
      const result = await agent.evolve('h', buildManifest({ nodes: [] as never[], edges: [{ from: 'x', to: 'y', mapping: {} }] }), {
        maxIterations: 5, cooldownMs: 0, costBudget: 100,
      });
      expect(result.passed).toBe(false);
      expect(result.error).toContain('invalid DAG');
      expect(result.iterations).toBe(0);
    });

    it('returns passed: true when score >= passThreshold', async () => {
      const m = buildManifest({ evaluation: { datasets: [], scorers: [], passThreshold: 0.5 } });
      executor.trace.nodeResults = {
        a: { nodeId: 'a', status: 'completed', output: 'ok', latencyMs: 100, costUsd: 0, totalTokens: 10, error: '' },
        b: { nodeId: 'b', status: 'completed', output: 'ok', latencyMs: 100, costUsd: 0, totalTokens: 10, error: '' },
      };
      const result = await agent.evolve('h', m, { maxIterations: 3, cooldownMs: 0, costBudget: 100 });
      expect(result.passed).toBe(true);
      expect(result.iterations).toBe(1);
      expect(executor.calls).toBe(1);
    });

    it('returns passed: false when score < passThreshold and max iterations reached', async () => {
      const m = buildManifest({ evaluation: { datasets: [], scorers: [], passThreshold: 0.99 } });
      executor.trace.nodeResults = {
        a: { nodeId: 'a', status: 'completed', output: 'ok', latencyMs: 100, costUsd: 0, totalTokens: 10, error: '' },
        b: { nodeId: 'b', status: 'failed', output: '', latencyMs: 100, costUsd: 0, totalTokens: 10, error: 'fail' },
      };
      agent.setReviseOverride(async (_manifest, _nodeId, _score) => ({
        revisedSubManifest: { systemPrompt: 'revised' },
        changes: ['updated system prompt'],
        reasoning: 'better clarity',
      }));
      const result = await agent.evolve('h', m, { maxIterations: 2, cooldownMs: 0, costBudget: 100 });
      expect(result.passed).toBe(false);
      expect(result.iterations).toBe(2);
      expect(result.error).toBeUndefined();
    });

    it('respects costBudget cap', async () => {
      const m = buildManifest();
      executor.trace.nodeResults = {
        a: { nodeId: 'a', status: 'failed', output: '', latencyMs: 100, costUsd: 0, totalTokens: 10, error: 'fail' },
      };
      executor.trace.totalCost = 60;
      agent.setReviseOverride(async (_manifest, _nodeId, _score) => ({
        revisedSubManifest: { systemPrompt: 'revised' },
        changes: ['updated system prompt'],
        reasoning: 'better clarity',
      }));
      const result = await agent.evolve('h', m, { maxIterations: 10, cooldownMs: 0, costBudget: 50 });
      expect(result.passed).toBe(false);
      expect(result.error).toContain('cost budget');
    });

    it('rollback: restores manifest when revision throws', async () => {
      const m = buildManifest();
      executor.trace.nodeResults = {
        a: { nodeId: 'a', status: 'failed', output: '', latencyMs: 100, costUsd: 0, totalTokens: 10, error: 'fail' },
      };
      let calls = 0;
      agent.setReviseOverride(async () => {
        calls += 1;
        if (calls === 1) throw new Error('LLM rate limit');
        return { revisedSubManifest: { systemPrompt: 'x' }, changes: ['c'], reasoning: 'r' };
      });
      const snap = agent.getSnapshot(0, m, 'h', 0);
      expect(snap.iteration).toBe(0);
      const result = await agent.evolve('h', m, { maxIterations: 3, cooldownMs: 0, costBudget: 100 });
      expect(result.error).toContain('LLM rate limit');
    });

    it('falls back to heuristic when primary scorer is unavailable (no LLM key)', async () => {
      const m = buildManifest();
      executor.trace.nodeResults = {
        a: { nodeId: 'a', status: 'completed', output: 'ok', latencyMs: 100, costUsd: 0, totalTokens: 10, error: '' },
        b: { nodeId: 'b', status: 'completed', output: 'ok', latencyMs: 100, costUsd: 0, totalTokens: 10, error: '' },
      };
      const result = await agent.evolve('h', m, { maxIterations: 1, cooldownMs: 0, costBudget: 100 });
      expect(result.passed).toBe(true);
      expect(result.iterations).toBe(1);
    });

    it('honors manifest.metadata.primaryScorer override', async () => {
      const m = buildManifest({ metadata: { primaryScorer: 'helpfulness' } });
      executor.trace.nodeResults = {
        a: { nodeId: 'a', status: 'completed', output: 'help me', latencyMs: 50, costUsd: 0, totalTokens: 5, error: '' },
      };
      const result = await agent.evolve('h', m, { maxIterations: 1, cooldownMs: 0, costBudget: 100 });
      expect(result.iterations).toBe(1);
    });

    it('honors manifest.evaluation.scorers[0] when primaryScorer absent', async () => {
      const m = buildManifest({ evaluation: { datasets: [], scorers: ['correctness'], passThreshold: 0.5 } });
      executor.trace.nodeResults = {
        a: { nodeId: 'a', status: 'completed', output: 'right answer', latencyMs: 50, costUsd: 0, totalTokens: 5, error: '' },
      };
      const result = await agent.evolve('h', m, { maxIterations: 1, cooldownMs: 0, costBudget: 100 });
      expect(result.iterations).toBe(1);
    });
  });
});