import { describe, it, expect } from 'vitest';
import { EvaluationAgent } from '../src/agents/evaluation/evaluation.js';
import type { AppConfig, EvalRun, DatasetCase } from '@promptsheon/shared';

function buildConfig(): AppConfig {
  return {
    server: { port: 8080, host: '127.0.0.1', dbPath: ':memory:', casPath: '/tmp/cas', frontendPath: '/tmp/web', corsOrigin: '', logLevel: 'info' },
    llm: { provider: 'openai', modelId: 'gpt-4', apiKeyEnv: 'OPENAI_API_KEY', maxRetries: 3, timeoutMs: 30000 },
    auth: { enabled: false, jwtSecret: '' },
    selfEvolve: { enabled: false, defaultCooldownSec: 900, maxConcurrentCycles: 3 },
  };
}

function makeCase(inputs: unknown, expected: unknown): DatasetCase {
  return {
    id: 'c1',
    datasetId: 'd1',
    seq: 0,
    inputs: JSON.stringify(inputs),
    expected: JSON.stringify(expected),
    description: '',
  };
}

class StubEvaluatorRegistry {
  evaluators: Map<string, { name: string; evaluate: (input: unknown) => Promise<{ score: number; passed: boolean; reasoning: string }> }>;
  constructor() {
    this.evaluators = new Map();
  }
  get(name: string) {
    const e = this.evaluators.get(name);
    if (!e) throw new Error(`unknown ${name}`);
    return e;
  }
}

describe('EvaluationAgent', () => {
  const config = buildConfig();
  const evalRun: EvalRun = {
    id: 'r1',
    releaseId: 'rel1',
    datasetId: 'd1',
    scorer: 'custom',
    score: 0,
    passed: 0,
    failed: 0,
    total: 0,
    status: 'running',
    startedAt: '2026-01-01T00:00:00Z',
    finishedAt: null,
  };

  it('aggregates scores across cases (100% pass)', async () => {
    const agent = new EvaluationAgent(config);
    // Override the internal registry with stubs that pass everything
    (agent as unknown as { evaluators: StubEvaluatorRegistry }).evaluators = new StubEvaluatorRegistry();
    (agent as unknown as { evaluators: StubEvaluatorRegistry }).evaluators.evaluators.set('custom', {
      name: 'custom',
      evaluate: async () => ({ score: 1, passed: true, reasoning: 'lgtm' }),
    });
    const cases = [makeCase({ a: 1 }, { b: 2 }), makeCase({ a: 3 }, { b: 4 })];
    const result = await agent.runEval(evalRun, cases, async () => 'ok');
    expect(result.score).toBe(1);
    expect(result.passed).toBe(2);
    expect(result.failed).toBe(0);
    expect(result.status).toBe('passed');
  });

  it('reports failed (0% pass)', async () => {
    const agent = new EvaluationAgent(config);
    (agent as unknown as { evaluators: StubEvaluatorRegistry }).evaluators = new StubEvaluatorRegistry();
    (agent as unknown as { evaluators: StubEvaluatorRegistry }).evaluators.evaluators.set('custom', {
      name: 'custom',
      evaluate: async () => ({ score: 0, passed: false, reasoning: 'no' }),
    });
    const cases = [makeCase({ a: 1 }, { b: 2 })];
    const result = await agent.runEval(evalRun, cases, async () => 'ok');
    expect(result.score).toBe(0);
    expect(result.passed).toBe(0);
    expect(result.failed).toBe(1);
    expect(result.status).toBe('failed');
  });

  it('handles empty case list (returns 0% score)', async () => {
    const agent = new EvaluationAgent(config);
    (agent as unknown as { evaluators: StubEvaluatorRegistry }).evaluators = new StubEvaluatorRegistry();
    (agent as unknown as { evaluators: StubEvaluatorRegistry }).evaluators.evaluators.set('custom', {
      name: 'custom',
      evaluate: async () => ({ score: 1, passed: true, reasoning: 'r' }),
    });
    const result = await agent.runEval(evalRun, [], async () => 'ok');
    expect(result.score).toBe(0);
    expect(result.total).toBe(0);
  });

  it('computes mixed pass rate (1/2 pass = 50%)', async () => {
    const agent = new EvaluationAgent(config);
    let count = 0;
    (agent as unknown as { evaluators: StubEvaluatorRegistry }).evaluators = new StubEvaluatorRegistry();
    (agent as unknown as { evaluators: StubEvaluatorRegistry }).evaluators.evaluators.set('custom', {
      name: 'custom',
      evaluate: async () => ({ score: 1, passed: count++ === 0, reasoning: 'r' }),
    });
    const cases = [makeCase({ a: 1 }, {}), makeCase({ a: 2 }, {})];
    const result = await agent.runEval(evalRun, cases, async () => 'ok');
    expect(result.passed).toBe(1);
    expect(result.failed).toBe(1);
    expect(result.score).toBe(0.5);
  });
});