import type { FastifyInstance } from 'fastify';
import type { SseHub } from '../sse/hub.js';
import { SseServerClient } from '../sse/client.js';

export function registerSseRoutes(app: FastifyInstance, sseHub: SseHub) {
  app.get('/api/events/:channel', async (request, reply) => {
    const { channel } = request.params as { channel: string };

    reply.raw.writeHead(200, {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-cache',
      Connection: 'keep-alive',
    });

    const clientId = crypto.randomUUID();
    const client = new SseServerClient(reply, clientId);
    sseHub.subscribe(channel, client);

    request.raw.on('close', () => {
      sseHub.unsubscribe(channel, client);
    });
  });
}
