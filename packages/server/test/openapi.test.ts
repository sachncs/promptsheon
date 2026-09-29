import { describe, expect, it } from 'vitest';
import Fastify from 'fastify';
import { registerFallbackRouteDoc, registerOpenApiRoutes, registerRouteDoc } from '../src/openapi.js';

describe('OpenAPI route registry', () => {
  it('keeps a fallback contract for routes without a hand-authored schema', async () => {
    const app = Fastify({ logger: false });
    app.addHook('onRoute', (routeOptions) => {
      if (!routeOptions.url.startsWith('/api/') || routeOptions.url === '/api/openapi.json') return;
      const methods = Array.isArray(routeOptions.method) ? routeOptions.method : [routeOptions.method];
      for (const method of methods) {
        if (method.toLowerCase() === 'get') registerFallbackRouteDoc('get', routeOptions.url);
      }
    });
    app.get('/api/example', async () => ({ ok: true }));
    registerOpenApiRoutes(app);
    await app.ready();

    const response = await app.inject({ method: 'GET', url: '/api/openapi.json' });
    expect(response.statusCode).toBe(200);
    expect(response.json<{ paths: Record<string, Record<string, unknown>> }>().paths['/api/example']?.get).toBeDefined();
    await app.close();
  });

  it('preserves a hand-authored contract over a fallback contract', async () => {
    const app = Fastify({ logger: false });
    app.addHook('onRoute', (routeOptions) => {
      if (routeOptions.url === '/api/example-rich') registerFallbackRouteDoc('get', routeOptions.url);
    });
    registerRouteDoc({
      method: 'get',
      path: '/api/example-rich',
      summary: 'Hand-authored summary',
      tags: ['example'],
    });
    app.get('/api/example-rich', async () => ({ ok: true }));
    registerOpenApiRoutes(app);
    await app.ready();

    const response = await app.inject({ method: 'GET', url: '/api/openapi.json' });
    expect(response.json<{ paths: Record<string, Record<string, { summary: string }>> }>().paths['/api/example-rich']?.get?.summary)
      .toBe('Hand-authored summary');
    await app.close();
  });
});
