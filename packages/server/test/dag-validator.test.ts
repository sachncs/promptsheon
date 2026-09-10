import { describe, it, expect } from 'vitest';
import { validateDag } from '../src/agents/executor/dag-validator.js';
import { buildNodeAgent, buildGraph, buildInvocationLimits } from '../src/agents/executor/node-builder.js';
import type { AppConfig, Manifest, SubCapabilityManifest } from '@promptsheon/shared';

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

function buildLeafManifest(id: string, version = 1): SubCapabilityManifest {
  return {
    id,
    name: id,
    description: '',
    goal: 'g',
    manifest: {
      id: `${id}-leaf`,
      version,
      prompt: { systemPrompt: 'x', userTemplate: '' },
      model: { provider: 'openai', modelId: 'gpt-4', temperature: 0.7, maxTokens: 100 },
      runtime: { timeoutMs: 1000, nodeTimeoutMs: 1000, totalTimeoutMs: 1000, maxRetries: 0, canaryPercent: 0, concurrencyLimit: 1 },
      context: { inputsSchema: {}, outputsSchema: {}, requiredContextVars: [] },
      memory: { enabled: false, type: 'stateless' },
      guardrails: { pre: [], post: [] },
      tools: [],
      mcpServers: [],
      evaluation: { datasets: [], scorers: [], passThreshold: 0.5 },
      nodes: [],
      edges: [],
      metadata: {},
      createdAt: '2026-01-01T00:00:00Z',
      updatedAt: '2026-01-01T00:00:00Z',
    },
    dependsOn: [],
    preGuardrails: [],
    postGuardrails: [],
    observability: { logInputs: true, logOutputs: true, trackLatency: true, trackCost: true },
    hooks: { beforeInvocation: false, afterInvocation: false, beforeModelCall: false, afterModelCall: false, beforeToolCall: false, afterToolCall: false },
    retry: { kind: 'exponential', maxAttempts: 3, baseDelayMs: 1000, maxDelayMs: 10000 },
    conversationManager: { kind: 'sliding-window', windowSize: 20 },
    state: { enabled: false, type: 'stateless' },
    limits: {},
  };
}

function buildTopManifest(overrides: Partial<Manifest> = {}): Manifest {
  return {
    id: 'm1',
    version: 1,
    prompt: { systemPrompt: 'x', userTemplate: '' },
    model: { provider: 'openai', modelId: 'gpt-4', temperature: 0.7, maxTokens: 100 },
    runtime: { timeoutMs: 1000, nodeTimeoutMs: 1000, totalTimeoutMs: 5000, maxRetries: 0, canaryPercent: 0, concurrencyLimit: 1 },
    context: { inputsSchema: {}, outputsSchema: {}, requiredContextVars: [] },
    memory: { enabled: false, type: 'stateless' },
    guardrails: { pre: [], post: [] },
    tools: [],
    mcpServers: [],
    evaluation: { datasets: [], scorers: [], passThreshold: 0.5 },
    nodes: [],
    edges: [],
    metadata: {},
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

describe('validateDag', () => {
  it('returns valid for a linear chain', () => {
    const m = buildTopManifest({
      nodes: [buildLeafManifest('a'), buildLeafManifest('b', 1)],
      edges: [{ from: 'a', to: 'b', mapping: {} }],
    });
    const result = validateDag(m);
    expect(result.valid).toBe(true);
    expect(result.errors).toEqual([]);
  });

  it('returns valid for parallel branches', () => {
    const m = buildTopManifest({
      nodes: [buildLeafManifest('a'), buildLeafManifest('b', 1), buildLeafManifest('c', 1)],
      edges: [
        { from: 'a', to: 'b', mapping: {} },
        { from: 'a', to: 'c', mapping: {} },
      ],
    });
    const result = validateDag(m);
    expect(result.valid).toBe(true);
  });

  it('detects duplicate node ids', () => {
    const m = buildTopManifest({
      nodes: [buildLeafManifest('a'), buildLeafManifest('a', 2)],
      edges: [],
    });
    const result = validateDag(m);
    expect(result.valid).toBe(false);
    expect(result.errors.some((e) => e.includes('duplicate'))).toBe(true);
  });

  it('detects self-loop', () => {
    const m = buildTopManifest({
      nodes: [buildLeafManifest('a')],
      edges: [{ from: 'a', to: 'a', mapping: {} }],
    });
    const result = validateDag(m);
    expect(result.valid).toBe(false);
    expect(result.errors.some((e) => e.includes('self-loop'))).toBe(true);
  });

  it('detects direct cycle', () => {
    const m = buildTopManifest({
      nodes: [buildLeafManifest('a'), buildLeafManifest('b', 1)],
      edges: [
        { from: 'a', to: 'b', mapping: {} },
        { from: 'b', to: 'a', mapping: {} },
      ],
    });
    const result = validateDag(m);
    expect(result.valid).toBe(false);
    expect(result.errors.some((e) => e.includes('cycle'))).toBe(true);
  });

  it('detects transitive cycle', () => {
    const m = buildTopManifest({
      nodes: [buildLeafManifest('a'), buildLeafManifest('b', 1), buildLeafManifest('c', 1)],
      edges: [
        { from: 'a', to: 'b', mapping: {} },
        { from: 'b', to: 'c', mapping: {} },
        { from: 'c', to: 'a', mapping: {} },
      ],
    });
    const result = validateDag(m);
    expect(result.valid).toBe(false);
  });

  it('detects dangling source edge', () => {
    const m = buildTopManifest({
      nodes: [buildLeafManifest('a')],
      edges: [{ from: 'nonexistent', to: 'a', mapping: {} }],
    });
    const result = validateDag(m);
    expect(result.valid).toBe(false);
    expect(result.errors.some((e) => e.includes('missing source'))).toBe(true);
  });

  it('detects dangling target edge', () => {
    const m = buildTopManifest({
      nodes: [buildLeafManifest('a')],
      edges: [{ from: 'a', to: 'nonexistent', mapping: {} }],
    });
    const result = validateDag(m);
    expect(result.valid).toBe(false);
    expect(result.errors.some((e) => e.includes('missing target'))).toBe(true);
  });
});

describe('buildNodeAgent', () => {
  it('constructs a Strands Agent from a SubCapabilityManifest', () => {
    const node = buildLeafManifest('test-node');
    const agent = buildNodeAgent(node, buildConfig());
    expect(agent).toBeDefined();
  });
});

describe('buildInvocationLimits', () => {
  it('returns undefined when no limits set', () => {
    expect(buildInvocationLimits({})).toBeUndefined();
  });

  it('returns object with only the set limits', () => {
    expect(buildInvocationLimits({ turns: 5, outputTokens: 1000 })).toEqual({ turns: 5, outputTokens: 1000 });
  });
});

describe('buildGraph', () => {
  it('builds a Strands Graph from a valid Manifest', () => {
    const m = buildTopManifest({
      nodes: [buildLeafManifest('a'), buildLeafManifest('b', 1)],
      edges: [{ from: 'a', to: 'b', mapping: {} }],
    });
    const graph = buildGraph(m, buildConfig());
    expect(graph).toBeDefined();
  });

  it('throws NotFoundError for an invalid DAG', () => {
    const m = buildTopManifest({
      nodes: [buildLeafManifest('a'), buildLeafManifest('a', 2)],
      edges: [],
    });
    expect(() => buildGraph(m, buildConfig())).toThrow();
  });
});