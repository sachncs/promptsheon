import { describe, it, expect } from 'vitest';
import type { AppConfig } from '@promptsheon/shared';
import { FailoverPolicy, getModelRegistry, failoverDescriptorFromCostCheck } from '../src/hardening/failover.js';
import { BudgetExceededError } from '../src/hardening/cost-caps.js';

const config: AppConfig = {
  server: { port: 8080, host: '127.0.0.1', dbPath: ':memory:', casPath: '/tmp/cas', frontendPath: '/tmp/web', corsOrigin: '', logLevel: 'info' },
  llm: { provider: 'openai', modelId: 'gpt-4', apiKeyEnv: 'OPENAI_API_KEY', maxRetries: 3, timeoutMs: 30000 },
  auth: { enabled: false, jwtSecret: '' },
  selfEvolve: { enabled: false, defaultCooldownSec: 900, maxConcurrentCycles: 3 },
};

describe('ModelRegistry', () => {
  it('seeds defaults for openai, anthropic, bedrock', () => {
    const reg = getModelRegistry();
    expect(reg.get('openai', 'gpt-4')).toBeDefined();
    expect(reg.get('openai', 'gpt-4o-mini')).toBeDefined();
    expect(reg.get('anthropic', 'claude-3-opus')).toBeDefined();
    expect(reg.get('anthropic', 'claude-3-haiku')).toBeDefined();
    expect(reg.get('bedrock', 'anthropic.claude-3-haiku')).toBeDefined();
  });

  it('listForProvider returns sorted by total cost (cheapest first)', () => {
    const reg = getModelRegistry();
    const openai = reg.listForProvider('openai');
    expect(openai.length).toBeGreaterThanOrEqual(3);
    for (let i = 1; i < openai.length; i++) {
      expect(reg.totalCost(openai[i]!)).toBeGreaterThanOrEqual(reg.totalCost(openai[i - 1]!));
    }
  });

  it('register() makes a new model discoverable', () => {
    const reg = getModelRegistry();
    reg.register({ provider: 'custom', modelId: 'cheap-1', costPerInputToken: 0.0000001, costPerOutputToken: 0.0000002 });
    expect(reg.get('custom', 'cheap-1')).toBeDefined();
  });
});

describe('FailoverPolicy', () => {
  const policy = new FailoverPolicy(config);

  it('picks a cheaper openai model when gpt-4 fails', () => {
    const next = policy.selectFailoverModel({ provider: 'openai', modelId: 'gpt-4' });
    expect(next).not.toBeNull();
    expect(next!.provider).toBe('openai');
    expect(next!.modelId).not.toBe('gpt-4');
    const reg = getModelRegistry();
    const cur = reg.get('openai', 'gpt-4');
    const nextCost = reg.get(next!.provider, next!.modelId);
    expect(cur).toBeDefined();
    expect(nextCost).toBeDefined();
    expect(reg.totalCost(nextCost!)).toBeLessThan(reg.totalCost(cur!));
  });

  it('picks a cheaper anthropic model when claude-3-opus fails', () => {
    const next = policy.selectFailoverModel({ provider: 'anthropic', modelId: 'claude-3-opus' });
    expect(next).not.toBeNull();
    expect(next!.provider).toBe('anthropic');
    expect(next!.modelId).not.toBe('claude-3-opus');
  });

  it('falls back to a cross-provider cheap model when same-provider has no cheaper', () => {
    const reg = getModelRegistry();
    reg.register({ provider: 'test', modelId: 'cheapest', costPerInputToken: 0, costPerOutputToken: 0 });
    const next = policy.selectFailoverModel({ provider: 'test', modelId: 'cheapest' });
    expect(next).not.toBeNull();
    expect(next!.provider).not.toBe('test');
  });

  it('returns null for unknown provider/model', () => {
    const next = policy.selectFailoverModel({ provider: 'unknown', modelId: 'unknown' });
    expect(next).toBeNull();
  });

  it('onBudgetExceeded only fires for per-invocation cap', () => {
    const perInvocation = new BudgetExceededError(5, 6, 'per-invocation');
    const orgDaily = new BudgetExceededError(100, 110, 'org-daily');
    expect(policy.onBudgetExceeded({ provider: 'openai', modelId: 'gpt-4' }, perInvocation)).not.toBeNull();
    expect(policy.onBudgetExceeded({ provider: 'openai', modelId: 'gpt-4' }, orgDaily)).toBeNull();
  });
});

describe('failoverDescriptorFromCostCheck', () => {
  it('parses "provider/modelId" notation', () => {
    const desc = failoverDescriptorFromCostCheck({ allowed: true, failoverModel: 'openai/gpt-3.5-turbo' });
    expect(desc).toEqual({ provider: 'openai', modelId: 'gpt-3.5-turbo' });
  });

  it('returns null when no failover model suggested', () => {
    expect(failoverDescriptorFromCostCheck({ allowed: true })).toBeNull();
  });

  it('returns null when notation is malformed', () => {
    expect(failoverDescriptorFromCostCheck({ allowed: true, failoverModel: 'malformed' })).toBeNull();
  });
});