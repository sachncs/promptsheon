import { describe, it, expect, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import { registerSseRoutes } from '../src/routes/sse.js';
import { SseHub } from '../src/sse/hub.js';

describe('SSE routes', () => {
  let app: FastifyInstance | undefined;
  let hub: SseHub | undefined;

  afterEach(async () => {
    hub?.destroy();
    await app?.close();
    hub = undefined;
    app = undefined;
  });

  it('hijacks the response before opening a streaming subscription', async () => {
    app = Fastify();
    hub = new SseHub();
    await registerSseRoutes(app, hub);
    const address = await app.listen({ host: '127.0.0.1', port: 0 });

    const controller = new AbortController();
    const response = await fetch(`${address}/api/events/contract-smoke`, { signal: controller.signal });

    expect(response.status).toBe(200);
    expect(response.headers.get('content-type')).toContain('text/event-stream');
    expect(hub.getClientCount('contract-smoke')).toBe(1);
    controller.abort();
  });
});
