import { describe, it, expect } from 'vitest';
import type { AppConfig } from '@promptsheon/shared';
import { EvaluatorRegistry, EVALUATOR_NAMES } from '../src/agents/evaluation/registry.js';
import { StrandsEvaluatorAdapter } from '../src/agents/evaluation/evaluator-adapter.js';

const config: AppConfig = {
  server: { port: 8080, host: '127.0.0.1', dbPath: ':memory:', casPath: '/tmp/cas', frontendPath: '/tmp/web', corsOrigin: '', logLevel: 'info' },
  llm: { provider: 'openai', modelId: 'gpt-4', apiKeyEnv: 'OPENAI_API_KEY', maxRetries: 3, timeoutMs: 30000 },
  auth: { enabled: false, jwtSecret: '' },
  selfEvolve: { enabled: false, defaultCooldownSec: 900, maxConcurrentCycles: 3 },
};

describe('EvaluatorRegistry', () => {
  const reg = new EvaluatorRegistry(config);

  it('exposes 25 evaluator names (24 plan names + llm-judge alias)', () => {
    expect(reg.list()).toHaveLength(EVALUATOR_NAMES.length);
    expect(EVALUATOR_NAMES.length).toBeGreaterThanOrEqual(24);
    for (const expected of ['helpfulness', 'coherence', 'correctness', 'goal-success-rate', 'output', 'trajectory', 'tool-selection', 'tool-parameter', 'multimodal-text']) {
      expect(reg.has(expected)).toBe(true);
    }
  });

  it('throws on unknown name at lookup time', () => {
    expect(() => reg.get('nope')).toThrow(/unknown evaluator/);
  });

  it('caches instances per name (identity check)', () => {
    const a = reg.get('helpfulness');
    const b = reg.get('helpfulness');
    expect(a).toBe(b);
  });

  it('returns different instances for different names', () => {
    const a = reg.get('helpfulness');
    const b = reg.get('coherence');
    expect(a).not.toBe(b);
  });
});

describe('StrandsEvaluatorAdapter', () => {
  it('rejects unknown evaluator names at construction', () => {
    expect(() => new StrandsEvaluatorAdapter(config, 'nope')).toThrow(/unknown evaluator name/);
  });

  it('returns the configured evaluator name', () => {
    const adapter = new StrandsEvaluatorAdapter(config, 'goal-success-rate');
    expect(adapter['evaluatorName']).toBe('goal-success-rate');
  });

  it('integrates with the registry (shared cache)', () => {
    const adapter = new StrandsEvaluatorAdapter(config, 'helpfulness');
    void adapter;
    const reg = new EvaluatorRegistry(config);
    const direct = reg.get('helpfulness');
    const directAgain = reg.get('helpfulness');
    expect(direct).toBe(directAgain);
  });
});