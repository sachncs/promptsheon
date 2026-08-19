import { describe, it, expect } from 'vitest';
import { IdeaPlannerAgent } from '../src/agents/planner/planner.js';
import type { AppConfig } from '@promptsheon/shared';

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

describe('IdeaPlannerAgent construction', () => {
  it('constructs without error', () => {
    const agent = new IdeaPlannerAgent(buildConfig());
    expect(agent).toBeInstanceOf(IdeaPlannerAgent);
  });
});

describe('IdeaPlannerAgent.plan fallback', () => {
  it('returns a single-node fallback DAG when given an empty idea (no LLM call made)', async () => {
    const agent = new IdeaPlannerAgent(buildConfig());
    const result = await agent.plan({ idea: '' });
    expect(result.goal).toBe('Process input');
    expect(result.nodes).toHaveLength(1);
    expect(result.nodes[0].id).toBe('root');
    expect(result.passThreshold).toBe(0.5);
  });
});