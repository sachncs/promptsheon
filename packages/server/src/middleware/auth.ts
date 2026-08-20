import type { FastifyRequest, FastifyReply } from 'fastify';
import { createHash } from 'node:crypto';
import type { AppConfig } from '@promptsheon/shared';
import type { ApiKeyRepo } from '../repos/api-key.js';

const BOOTSTRAP_PREFIX = '/api/bootstrap/';

export function authMiddleware(config: AppConfig, apiKeyRepo: ApiKeyRepo) {
  return async (request: FastifyRequest, reply: FastifyReply) => {
    if (request.url.startsWith(BOOTSTRAP_PREFIX)) {
      (request as unknown as Record<string, string>).userId = 'bootstrap';
      (request as unknown as { orgContextBypass?: boolean }).orgContextBypass = true;
      return;
    }

    if (!config.auth.enabled) {
      (request as unknown as Record<string, string>).userId = 'api';
      return;
    }

    const authHeader = request.headers.authorization;
    if (!authHeader) {
      return reply.code(401).send({ error: { code: 'UNAUTHORIZED', message: 'Missing authorization header' } });
    }

    if (authHeader.startsWith('Bearer ')) {
      const token = authHeader.slice(7);
      const keyHash = createHash('sha256').update(token).digest('hex');
      const apiKey = await apiKeyRepo.findByKeyHash(keyHash);

      if (!apiKey || apiKey.revoked) {
        return reply.code(401).send({ error: { code: 'UNAUTHORIZED', message: 'Invalid API key' } });
      }

      if (apiKey.expiresAt && new Date(apiKey.expiresAt) < new Date()) {
        return reply.code(401).send({ error: { code: 'UNAUTHORIZED', message: 'API key expired' } });
      }

      (request as unknown as Record<string, string>).userId = apiKey.userId;
      (request as unknown as Record<string, string>).userRole = apiKey.role;
      return;
    }

    return reply.code(401).send({ error: { code: 'UNAUTHORIZED', message: 'Invalid authorization format' } });
  };
}
