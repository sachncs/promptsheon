import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { runGuardrail, runAllGuardrails } from '../src/agents/executor/guardrails.js';
import { SseHub } from '../src/sse/hub.js';
import type { GuardrailSpec, Manifest } from '@promptsheon/shared';
import type { AppConfig } from '@promptsheon/shared';

function buildConfig(): AppConfig {
  return {
    server: {
      port: 8080, host: '127.0.0.1', dbPath: ':memory:', casPath: '/tmp/cas',
      frontendPath: '/tmp/web', corsOrigin: '', logLevel: 'info',
    },
    llm: { provider: 'openai', modelId: 'gpt-4', apiKeyEnv: 'OPENAI_API_KEY', maxRetries: 3, timeoutMs: 30000 },
    auth: { enabled: false, jwtSecret: '' },
    selfEvolve: { enabled: false, defaultCooldownSec: 900, maxConcurrentCycles: 3 },
  };
}

function buildManifest(): Manifest {
  return {
    id: 'm', version: 1,
    prompt: { systemPrompt: '', userTemplate: '' },
    model: { provider: 'openai', modelId: 'gpt-4', temperature: 0.7, maxTokens: 100 },
    runtime: { timeoutMs: 1000, nodeTimeoutMs: 1000, totalTimeoutMs: 1000, maxRetries: 0, canaryPercent: 0, concurrencyLimit: 1 },
    context: { inputsSchema: {}, outputsSchema: {}, requiredContextVars: [] },
    memory: { enabled: false, type: 'stateless' },
    guardrails: { pre: [], post: [] },
    tools: [], mcpServers: [],
    evaluation: { datasets: [], scorers: [], passThreshold: 0.5 },
    nodes: [], edges: [], metadata: {},
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  };
}

describe('runGuardrail: regex', () => {
  it('returns null when no match', () => {
    const spec: GuardrailSpec = { type: 'regex', config: { pattern: 'forbidden' }, enabled: true, onFailure: 'block' };
    expect(runGuardrail(spec, { manifest: buildManifest(), nodeId: 'n', executionId: 'e', values: ['hello world'], phase: 'pre' })).toBeNull();
  });

  it('returns failure when match', () => {
    const spec: GuardrailSpec = { type: 'regex', config: { pattern: 'forbidden' }, enabled: true, onFailure: 'block' };
    expect(runGuardrail(spec, { manifest: buildManifest(), nodeId: 'n', executionId: 'e', values: ['this is forbidden content'], phase: 'pre' })).toContain('forbidden');
  });

  it('respects enabled: false', () => {
    const spec: GuardrailSpec = { type: 'regex', config: { pattern: 'forbidden' }, enabled: false, onFailure: 'block' };
    expect(runGuardrail(spec, { manifest: buildManifest(), nodeId: 'n', executionId: 'e', values: ['forbidden'], phase: 'pre' })).toBeNull();
  });
});

describe('runGuardrail: blocklist', () => {
  it('matches any blocklist term in a string', () => {
    const spec: GuardrailSpec = { type: 'blocklist', config: { terms: ['bad', 'evil'] }, enabled: true, onFailure: 'block' };
    expect(runGuardrail(spec, { manifest: buildManifest(), nodeId: 'n', executionId: 'e', values: ['this is evil'], phase: 'pre' })).toContain('evil');
  });

  it('is case-insensitive', () => {
    const spec: GuardrailSpec = { type: 'blocklist', config: { terms: ['bad'] }, enabled: true, onFailure: 'block' };
    expect(runGuardrail(spec, { manifest: buildManifest(), nodeId: 'n', executionId: 'e', values: ['BAD things'], phase: 'pre' })).toContain('bad');
  });
});

describe('runGuardrail: schema', () => {
  it('passes when keys match allowlist', () => {
    const spec: GuardrailSpec = { type: 'schema', config: { allowedKeys: ['name', 'age'] }, enabled: true, onFailure: 'block' };
    expect(runGuardrail(spec, { manifest: buildManifest(), nodeId: 'n', executionId: 'e', values: [{ name: 'x', age: 1 }], phase: 'pre' })).toBeNull();
  });

  it('rejects extra keys', () => {
    const spec: GuardrailSpec = { type: 'schema', config: { allowedKeys: ['name'] }, enabled: true, onFailure: 'block' };
    expect(runGuardrail(spec, { manifest: buildManifest(), nodeId: 'n', executionId: 'e', values: [{ name: 'x', extra: 1 }], phase: 'pre' })).toContain('extra');
  });
});

describe('runGuardrail: pii-redaction', () => {
  it('redacts SSN, card, email and returns null (no failure)', () => {
    const spec: GuardrailSpec = { type: 'pii-redaction', config: {}, enabled: true, onFailure: 'redact' };
    const values = ['SSN: 123-45-6789, card 4111-1111-1111-1111, email me@a.com'];
    const result = runGuardrail(spec, { manifest: buildManifest(), nodeId: 'n', executionId: 'e', values, phase: 'pre' });
    expect(result).toBeNull();
    expect(values[0]).toContain('[SSN]');
    expect(values[0]).toContain('[CARD]');
    expect(values[0]).toContain('[EMAIL]');
  });
});

describe('runGuardrail: llm-judge', () => {
  it('always passes (would require async LLM)', () => {
    const spec: GuardrailSpec = { type: 'llm-judge', config: {}, enabled: true, onFailure: 'block' };
    expect(runGuardrail(spec, { manifest: buildManifest(), nodeId: 'n', executionId: 'e', values: ['anything'], phase: 'pre' })).toBeNull();
  });
});

describe('runAllGuardrails', () => {
  let hub: SseHub;
  beforeEach(() => { hub = new SseHub(); });
  afterEach(() => { hub.stop?.(); });

  it('returns allowed: true when no guardrails', () => {
    const result = runAllGuardrails([], { manifest: buildManifest(), nodeId: 'n', executionId: 'e', values: ['x'], phase: 'pre' }, { hub, config: buildConfig() });
    expect(result.allowed).toBe(true);
  });

  it('returns allowed: false when a blocking guardrail fails', () => {
    const specs: GuardrailSpec[] = [
      { type: 'blocklist', config: { terms: ['forbidden'] }, enabled: true, onFailure: 'block' },
    ];
    const result = runAllGuardrails(specs, { manifest: buildManifest(), nodeId: 'n', executionId: 'e', values: ['this is forbidden'], phase: 'pre' }, { hub, config: buildConfig() });
    expect(result.allowed).toBe(false);
  });

  it('allows execution when guardrail is warn (not block)', () => {
    const specs: GuardrailSpec[] = [
      { type: 'blocklist', config: { terms: ['bad'] }, enabled: true, onFailure: 'warn' },
    ];
    const result = runAllGuardrails(specs, { manifest: buildManifest(), nodeId: 'n', executionId: 'e', values: ['bad'], phase: 'pre' }, { hub, config: buildConfig() });
    expect(result.allowed).toBe(true);
  });
});