import { describe, it, expect } from 'vitest';
import Fastify from 'fastify';
import { registerCompilerRoutes } from '../src/routes/compiler.js';

class StubCompiler {
  async compile(_manifest: unknown, opts: unknown) {
    return { manifest: { compiled: true }, opts };
  }
  async decompile(_manifest: unknown) {
    return { prompt: 'decompiled-prompt' };
  }
}

const manifest = {
  id: 'manifest-1',
  version: 1,
  prompt: { systemPrompt: 'a raw prompt to compile', userTemplate: '{{input}}' },
  model: { provider: 'openai', modelId: 'gpt-4', temperature: 0.7, maxTokens: 4096 },
  runtime: { timeoutMs: 30000, nodeTimeoutMs: 10000, totalTimeoutMs: 300000, maxRetries: 3, canaryPercent: 0, concurrencyLimit: 10 },
  context: { inputsSchema: {}, outputsSchema: {}, requiredContextVars: [] },
  memory: { enabled: false, type: 'stateless' },
  guardrails: { pre: [], post: [] },
  tools: [],
  mcpServers: [],
  evaluation: { datasets: [], scorers: [], passThreshold: 0.7 },
  nodes: [],
  edges: [],
  metadata: {},
};

function createApp() {
  const app = Fastify({ logger: false });
  registerCompilerRoutes(app, new StubCompiler() as never);
  return app;
}

describe('compiler route', () => {
  it('accepts a validated manifest and routes to compile()', async () => {
    const app = createApp();
    await app.ready();
    const response = await app.inject({ method: 'POST', url: '/api/compiler/compile', payload: { manifest } });
    expect(response.statusCode).toBe(200);
    expect((response.json() as { manifest: { compiled: boolean } }).manifest.compiled).toBe(true);
  });

  it('passes compiler context and constraints', async () => {
    const app = createApp();
    await app.ready();
    const response = await app.inject({
      method: 'POST',
      url: '/api/compiler/compile',
      payload: { manifest, capabilityContext: 'ctx', constraints: ['c1'] },
    });
    expect(response.statusCode).toBe(200);
  });

  it('returns 422 when manifest is missing', async () => {
    const app = createApp();
    await app.ready();
    const response = await app.inject({ method: 'POST', url: '/api/compiler/compile', payload: {} });
    expect(response.statusCode).toBe(422);
  });

  it('rejects the removed prompt-only contract', async () => {
    const app = createApp();
    await app.ready();
    const response = await app.inject({ method: 'POST', url: '/api/compiler/compile', payload: { prompt: 'raw prompt' } });
    expect(response.statusCode).toBe(422);
  });
});
