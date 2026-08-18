import type { FastifyRequest, FastifyReply } from 'fastify';
import type { AuditChain } from '../audit/chain.js';

export function auditMiddleware(auditChain: AuditChain) {
  return async (request: FastifyRequest, reply: FastifyReply) => {
    const startTime = Date.now();

    reply.raw.on('finish', () => {
      if (request.method === 'GET') return;

      auditChain.append({
        userId: (request as unknown as Record<string, string>).userId ?? 'anonymous',
        action: `${request.method} ${request.url}`,
        resource: request.url.split('/')[2] ?? 'unknown',
        details: JSON.stringify({
          statusCode: reply.statusCode,
          latencyMs: Date.now() - startTime,
        }),
        resourceKind: request.url.split('/')[2] ?? 'unknown',
        resourceId: (request.params as Record<string, string>)?.id ?? '',
      });
    });
  };
}
