import { describe, it, expect } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import { registerCompilerRoutes } from '../src/routes/compiler.js';

class StubCompiler {
  async compile(_manifest: unknown, opts: unknown) {
    return { manifest: { compiled: true }, opts };
  }
  async decompile(_manifest: unknown) {
    return { prompt: 'decompiled-prompt' };
  }
}

describe('compiler route legacy {prompt} acceptance', () => {
  it('accepts {prompt: string} and routes to compile()', async () => {
    const app = Fastify({ logger: false });
    registerCompilerRoutes(app, new StubCompiler() as unknown as Parameters<typeof registerCompilerRoutes>[1]);
    await app.ready();
    const r = await app.inject({
      method: 'POST',
      url: '/api/compiler/compile',
      payload: { prompt: 'a raw prompt to compile' },
    });
    expect(r.statusCode).toBe(200);
    const body = r.json() as { manifest: { compiled: boolean } };
    expect(body.manifest.compiled).toBe(true);
  });

  it('still accepts the legacy {manifest, capabilityContext, constraints} shape', async () => {
    const app = Fastify({ logger: false });
    registerCompilerRoutes(app, new StubCompiler() as unknown as Parameters<typeof registerCompilerRoutes>[1]);
    await app.ready();
    const r = await app.inject({
      method: 'POST',
      url: '/api/compiler/compile',
      payload: { manifest: { id: 'x' }, capabilityContext: 'ctx', constraints: ['c1'] },
    });
    expect(r.statusCode).toBe(200);
  });

  it('returns 422 when neither manifest nor prompt is provided', async () => {
    const app = Fastify({ logger: false });
    registerCompilerRoutes(app, new StubCompiler() as unknown as Parameters<typeof registerCompilerRoutes>[1]);
    await app.ready();
    const r = await app.inject({ method: 'POST', url: '/api/compiler/compile', payload: {} });
    expect(r.statusCode).toBe(422);
  });

  it('returns 422 when prompt is empty string', async () => {
    const app = Fastify({ logger: false });
    registerCompilerRoutes(app, new StubCompiler() as unknown as Parameters<typeof registerCompilerRoutes>[1]);
    await app.ready();
    const r = await app.inject({ method: 'POST', url: '/api/compiler/compile', payload: { prompt: '' } });
    expect(r.statusCode).toBe(422);
  });
});
