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

    // Take ownership of the raw response before the SSE client writes its
    // headers. Without hijacking, Fastify continues the normal reply
    // lifecycle after this handler returns and can attempt to write a second
    // response, producing ERR_HTTP_HEADERS_SENT for every subscription.
    reply.hijack();

    const clientId = crypto.randomUUID();
    const client = new SseServerClient(reply, clientId);
    sseHub.subscribe(channel, client);

    request.raw.on('close', () => {
      sseHub.unsubscribe(channel, client);
    });
  });
}
