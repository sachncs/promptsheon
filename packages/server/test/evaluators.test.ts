import { describe, it, expect } from 'vitest';
import { buildEvaluatorRegistry, listEvaluators, getEvaluator } from '../src/evaluation/evaluators.js';
import type { AppConfig } from '@promptsheon/shared';

function buildConfig(): AppConfig {
  return {
    server: { port: 8080, host: '127.0.0.1', dbPath: ':memory:', casPath: '/tmp/cas', frontendPath: '/tmp/web', corsOrigin: '', logLevel: 'info' },
    llm: { provider: 'openai', modelId: 'gpt-4', apiKeyEnv: 'OPENAI_API_KEY', maxRetries: 3, timeoutMs: 30000 },
    auth: { enabled: false, jwtSecret: '' },
    selfEvolve: { enabled: false, defaultCooldownSec: 900, maxConcurrentCycles: 3 },
  };
}

describe('buildEvaluatorRegistry', () => {
  it('builds registry with 6 evaluators', () => {
    const reg = buildEvaluatorRegistry(buildConfig());
    expect(reg.size).toBe(6);
  });

  it('evaluators include the credential-free deterministic scorer', () => {
    const reg = buildEvaluatorRegistry(buildConfig());
    expect(listEvaluators(reg).sort()).toEqual([
      'coherence', 'correctness', 'deterministic', 'goal-success-rate', 'helpfulness', 'llm-judge',
    ]);
  });

  it('getEvaluator returns the right instance', () => {
    const reg = buildEvaluatorRegistry(buildConfig());
    expect(getEvaluator(reg, 'helpfulness').name).toBe('helpfulness');
  });

  it('getEvaluator throws for unknown name', () => {
    const reg = buildEvaluatorRegistry(buildConfig());
    expect(() => getEvaluator(reg, 'unknown')).toThrow(/unknown evaluator/);
  });
});
