import type { FastifyRequest, FastifyReply } from 'fastify';
import type { IdempotencyRepo } from '../repos/idempotency.js';
import { createHash } from 'node:crypto';

export function idempotencyMiddleware(idempotencyRepo: IdempotencyRepo) {
  return async (request: FastifyRequest, reply: FastifyReply) => {
    const idempotencyKey = request.headers['idempotency-key'] as string | undefined;
    if (!idempotencyKey) return;

    const key = createHash('sha256')
      .update(`${request.method}:${request.url}:${idempotencyKey}`)
      .digest('hex');

    const cached = await idempotencyRepo.get(key);
    if (cached) {
      return reply
        .code(cached.statusCode)
        .headers(JSON.parse(cached.headers))
        .send(cached.body);
    }

    reply.raw.on('finish', () => {
      idempotencyRepo.set(key, reply.statusCode, JSON.stringify(reply.getHeaders()), Buffer.alloc(0), new Date(Date.now() + 86_400_000).toISOString());
    });
  };
}
