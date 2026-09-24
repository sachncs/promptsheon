import type { FastifyInstance } from 'fastify';
import type { SseHub } from '../sse/hub.js';
import { SseServerClient } from '../sse/client.js';
import { z } from 'zod';
import { parseParams } from './validate.js';

const SseChannelParamsSchema = z.object({
  channel: z.string().trim().min(1).max(255),
});

export function registerSseRoutes(app: FastifyInstance, sseHub: SseHub) {
  app.get('/api/events/:channel', async (request, reply) => {
    const parsedParams = parseParams(reply, SseChannelParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { channel } = parsedParams.data;

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
