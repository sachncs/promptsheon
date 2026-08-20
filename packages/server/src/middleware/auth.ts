import type { FastifyRequest, FastifyReply } from 'fastify';
import { createHash } from 'node:crypto';
import type { AppConfig } from '@promptsheon/shared';
import type { ApiKeyRepo } from '../repos/api-key.js';

const BOOTSTRAP_PREFIX = '/api/bootstrap/';
const PUBLIC_PATHS = new Set([
  '/api/openapi.json',
  '/api/health',
]);

/**
 * Auth middleware — Bearer tokens first, X-User-Id fallback.
 *
 * Bearer tokens are sha256-hashed in the api_keys table and
 * resolve to an org-scoped user. The SDK + CLI issue Bearer
 * tokens; older internal callers can still pass X-User-Id +
 * X-Org-Id during tests.
 */
export function authMiddleware(config: AppConfig, apiKeyRepo: ApiKeyRepo) {
  return async (request: FastifyRequest, reply: FastifyReply) => {
    if (request.url.startsWith(BOOTSTRAP_PREFIX)) {
      (request as unknown as Record<string, string>).userId = 'bootstrap';
      (request as unknown as { orgContextBypass?: boolean }).orgContextBypass = true;
      return;
    }
    if (PUBLIC_PATHS.has(request.url.split('?')[0] ?? '')) {
      (request as unknown as Record<string, string>).userId = 'public';
      (request as unknown as { orgContextBypass?: boolean }).orgContextBypass = true;
      return;
    }

    if (!config.auth.enabled) {
      const headerUser = request.headers['x-user-id'];
      if (typeof headerUser === 'string' && headerUser.length > 0) {
        (request as unknown as Record<string, string>).userId = headerUser;
      } else {
        (request as unknown as Record<string, string>).userId = 'api';
      }
      return;
    }

    const authHeader = request.headers.authorization;
    if (authHeader && authHeader.startsWith('Bearer ')) {
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
      void apiKeyRepo.updateLastUsed(apiKey.id);
      return;
    }

    // Legacy fallback so internal callers (dev tools, admin
    // probes) can still pass X-User-Id + X-Org-Id during tests.
    const headerUser = request.headers['x-user-id'];
    if (typeof headerUser === 'string' && headerUser.length > 0) {
      (request as unknown as Record<string, string>).userId = headerUser;
      return;
    }

    return reply.code(401).send({ error: { code: 'UNAUTHORIZED', message: 'Missing authorization header' } });
  };
}
