import { describe, expect, it, vi } from 'vitest';
import type { Manifest } from '@promptsheon/shared';
import { ExecutionService } from '../../src/application/execution-service.js';

const manifest: Manifest = {
  id: 'manifest-1',
  version: 1,
  prompt: { systemPrompt: 'system', userTemplate: '{{input}}' },
  model: { provider: 'openai', modelId: 'gpt-4', temperature: 0.2, maxTokens: 100 },
  runtime: {
    timeoutMs: 1_000,
    nodeTimeoutMs: 1_000,
    totalTimeoutMs: 5_000,
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
  metadata: { capabilityId: 'capability-1' },
  createdAt: '2026-01-01T00:00:00.000Z',
  updatedAt: '2026-01-01T00:00:00.000Z',
};

function dependencies() {
  const traces = {
    startRun: vi.fn(() => ({ id: 'trace-1' })),
    finalize: vi.fn(),
  };
  const records = { create: vi.fn() };
  const runner = {
    execute: vi.fn(async () => ({
      executionId: 'execution-1',
      manifestHash: 'hash-1',
      status: 'completed' as const,
      startedAt: '2026-01-01T00:00:00.000Z',
      endedAt: '2026-01-01T00:00:01.000Z',
      nodeResults: { 'node-1': { output: 'done' } },
      totalCost: 0.01,
      totalLatencyMs: 100,
      totalTokens: 12,
    })),
  };
  const service = new ExecutionService(
    { findByHashInOrg: vi.fn(() => manifest) },
    { findActiveByManifestHashInOrg: vi.fn(() => [{ id: 'release-1', canaryPercent: 100 }]) },
    traces,
    records,
    runner,
    vi.fn(() => 'release-1'),
  );
  return { service, traces, records, runner };
}

describe('ExecutionService', () => {
  it('does not invoke infrastructure when the manifest is outside the organization', async () => {
    const deps = dependencies();
    const manifests = { findByHashInOrg: vi.fn(() => null) };
    const service = new ExecutionService(
      manifests,
      { findActiveByManifestHashInOrg: vi.fn() },
      deps.traces,
      deps.records,
      deps.runner,
      vi.fn(),
    );

    await expect(service.run('hash-1', 'org-b', {
      executionId: 'execution-1',
      inputs: {},
      environment: 'prod',
    })).resolves.toEqual({ kind: 'manifest-not-found' });
    expect(deps.runner.execute).not.toHaveBeenCalled();
    expect(deps.traces.startRun).not.toHaveBeenCalled();
  });

  it('returns a no-active-release result without opening a trace', async () => {
    const deps = dependencies();
    const service = new ExecutionService(
      { findByHashInOrg: vi.fn(() => manifest) },
      { findActiveByManifestHashInOrg: vi.fn(() => []) },
      deps.traces,
      deps.records,
      deps.runner,
      vi.fn(() => null),
    );

    await expect(service.run('hash-1', 'org-a', {
      executionId: 'execution-1',
      inputs: {},
      environment: 'prod',
    })).resolves.toEqual({ kind: 'no-active-release' });
    expect(deps.traces.startRun).not.toHaveBeenCalled();
  });

  it('coordinates execution, trace finalization, and persistence', async () => {
    const deps = dependencies();
    const result = await deps.service.run('hash-1', 'org-a', {
      executionId: 'execution-1',
      inputs: { prompt: 'hello' },
      environment: 'prod',
      traceId: 'trace-external',
    });

    expect(result.kind).toBe('success');
    expect(deps.traces.startRun).toHaveBeenCalledWith(expect.objectContaining({ organizationId: 'org-a' }));
    expect(deps.runner.execute).toHaveBeenCalledWith(
      'hash-1',
      manifest,
      expect.objectContaining({ traceRunId: 'trace-1', traceId: 'trace-external' }),
    );
    expect(deps.traces.finalize).toHaveBeenCalledWith('trace-1', 'success', { tokens: 12, costUsd: 0.01 });
    expect(deps.records.create).toHaveBeenCalledWith(expect.objectContaining({ traceId: 'trace-external' }));
  });

  it('finalizes a failed trace before rethrowing runner errors', async () => {
    const deps = dependencies();
    const failure = new Error('executor unavailable');
    deps.runner.execute.mockRejectedValue(failure);

    await expect(deps.service.run('hash-1', 'org-a', {
      executionId: 'execution-1',
      inputs: {},
      environment: 'prod',
    })).rejects.toThrow('executor unavailable');
    expect(deps.traces.finalize).toHaveBeenCalledWith('trace-1', 'error', { tokens: 0, costUsd: 0 });
    expect(deps.records.create).not.toHaveBeenCalled();
  });
});
