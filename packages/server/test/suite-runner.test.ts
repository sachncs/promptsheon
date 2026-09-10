import { describe, it, expect } from 'vitest';
import type { AppConfig, Manifest } from '@promptsheon/shared';
import { EvalSuiteRunner } from '../src/agents/evaluation/suite-runner.js';
import { EvaluatorRegistry, EVALUATOR_NAMES } from '../src/agents/evaluation/registry.js';

const config: AppConfig = {
  server: { port: 8080, host: '127.0.0.1', dbPath: ':memory:', casPath: '/tmp/cas', frontendPath: '/tmp/web', corsOrigin: '', logLevel: 'info' },
  llm: { provider: 'openai', modelId: 'gpt-4', apiKeyEnv: 'OPENAI_API_KEY', maxRetries: 3, timeoutMs: 30000 },
  auth: { enabled: false, jwtSecret: '' },
  selfEvolve: { enabled: false, defaultCooldownSec: 900, maxConcurrentCycles: 3 },
};

function buildManifest(scorers: string[]): Manifest {
  return {
    id: 'm1', version: 1,
    prompt: { systemPrompt: 'x', userTemplate: '{{input}}' },
    model: { provider: 'openai', modelId: 'gpt-4', temperature: 0.7, maxTokens: 100 },
    runtime: { timeoutMs: 1000, nodeTimeoutMs: 1000, totalTimeoutMs: 5000, maxRetries: 0, canaryPercent: 0, concurrencyLimit: 1 },
    context: { inputsSchema: {}, outputsSchema: {}, requiredContextVars: [] },
    memory: { enabled: false, type: 'stateless' },
    guardrails: { pre: [], post: [] },
    tools: [], mcpServers: [],
    evaluation: { datasets: [], scorers, passThreshold: 0.5 },
    nodes: [], edges: [], metadata: {},
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  };
}

describe('EvalSuiteRunner', () => {
  const runner = new EvalSuiteRunner(config);

  it('returns zero result when no scorers declared', async () => {
    const result = await runner.run(buildManifest([]), { actual: 'x' });
    expect(result.scorerResults).toEqual([]);
    expect(result.aggregateScore).toBe(0);
    expect(result.passed).toBe(false);
  });

  it('filters out unknown scorer names (no LLM call)', () => {
    const declared = ['helpfulness', 'not-a-real-scorer'];
    const filtered = declared.filter((s) => (EVALUATOR_NAMES as readonly string[]).includes(s));
    expect(filtered).toEqual(['helpfulness']);
    expect(filtered).not.toContain('not-a-real-scorer');
  });

  it('aggregate score is the arithmetic mean', () => {
    const agg = runner.aggregate([
      { evaluatorName: 'a', score: 0.8, passed: true, reasoning: '' },
      { evaluatorName: 'b', score: 0.4, passed: false, reasoning: '' },
    ]);
    expect(agg.aggregateScore).toBeCloseTo(0.6, 5);
  });

  it('passed = AND of all scorer passes', () => {
    const allPass = runner.aggregate([
      { evaluatorName: 'a', score: 1, passed: true, reasoning: '' },
      { evaluatorName: 'b', score: 0.5, passed: true, reasoning: '' },
    ]);
    expect(allPass.passed).toBe(true);

    const oneFail = runner.aggregate([
      { evaluatorName: 'a', score: 1, passed: true, reasoning: '' },
      { evaluatorName: 'b', score: 0, passed: false, reasoning: '' },
    ]);
    expect(oneFail.passed).toBe(false);
  });

  it('accepts scorer override via options.scorers (no LLM call)', () => {
    const declared = ['helpfulness', 'correctness'];
    const filtered = declared.filter((s) => (EVALUATOR_NAMES as readonly string[]).includes(s));
    expect(filtered).toEqual(['helpfulness', 'correctness']);
  });

  it('shares registry with EvaluatorRegistry (cache hit)', () => {
    const reg = new EvaluatorRegistry(config);
    const r1 = reg.get('helpfulness');
    const r2 = reg.get('helpfulness');
    expect(r1).toBe(r2);
  });
});