import { describe, expect, it } from 'vitest';
import { ManifestSchema, type AppConfig } from '@promptsheon/shared';
import { ReasoningCompiler } from '../src/agents/compiler/compiler.js';

const config: AppConfig = {
  server: {
    port: 8080,
    host: '127.0.0.1',
    dbPath: ':memory:',
    casPath: '/tmp/promptsheon-test-cas',
    frontendPath: '',
    corsOrigin: '',
    logLevel: 'silent',
    nodeEnv: 'test',
  },
  llm: {
    defaultProvider: 'simulated',
    defaultModel: 'promptsheon-simulator',
    apiKeyEnvVar: 'PROMPTSHEON_API_KEY',
    maxRetries: 0,
    timeoutMs: 1_000,
  },
  auth: { enabled: false, jwtSecret: '' },
  selfEvolve: { enabled: false, defaultCooldownSec: 0, maxConcurrent: 1 },
};

describe('reasoning compiler runtime', () => {
  it('compiles deterministically without an LLM credential in simulator mode', async () => {
    const manifest = ManifestSchema.parse({
      id: 'manifest-1',
      version: 1,
      prompt: { systemPrompt: 'a raw prompt', userTemplate: '{{input}}' },
      model: { provider: 'simulated', modelId: 'promptsheon-simulator', temperature: 0.7, maxTokens: 512 },
      runtime: { timeoutMs: 30_000, nodeTimeoutMs: 10_000, totalTimeoutMs: 300_000, maxRetries: 0, canaryPercent: 0, concurrencyLimit: 1 },
      context: { inputsSchema: {}, outputsSchema: {}, requiredContextVars: [] },
      memory: { enabled: false, type: 'stateless' },
      guardrails: { pre: [], post: [] },
      tools: [],
      mcpServers: [],
      evaluation: { datasets: [], scorers: [], passThreshold: 0.7 },
      nodes: [],
      edges: [],
      metadata: {},
    });
    const compiler = new ReasoningCompiler(config);

    await expect(compiler.compile(manifest, {
      capabilityContext: 'credential-free test',
      constraints: ['preserve output shape'],
    })).resolves.toEqual(manifest);
  });
});
