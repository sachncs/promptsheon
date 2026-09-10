import type { FastifyRequest, FastifyReply } from 'fastify';
import { createHash } from 'node:crypto';
import type { AppConfig } from '@promptsheon/shared';
import type { ApiKeyRepo } from '../repos/api-key.js';
import { verifySVID } from '../identity/svid.js';

const BOOTSTRAP_PREFIX = '/api/bootstrap/';
const PUBLIC_PATHS = new Set([
  '/api/openapi.json',
  '/api/health',
  '/api/audit/verify',
  '/api/audit/state',
]);

/**
 * Auth middleware — Bearer + SVID; legacy X-User-Id fallback
 * is only honoured when auth is *disabled*.
 *
 *  - `Authorization: Bearer <token>` → sha256 lookup in api_keys.
 *  - `Authorization: SVID <token>`   → ed25519 verification against
 *    the operator signing key (env var
 *    `PROMPTSHEON_SVID_PUBLIC_KEY_PEM` for v1; the per-org signing
 *    key lookup wires in through SigningKeyRepo in AG-7). On
 *    success the request is stamped with `principal: 'Agent'`
 *    + the SVID subject + org + classification so the Cedar
 *    gate can authorize the action.
 *
 * When auth is enabled (the production default), any request
 * without a Bearer / SVID header is rejected with 401 — the
 * legacy X-User-Id fallback is intentionally NOT honoured, since
 * it bypasses every maker-checker / approval / audit chain that
 * depends on the request's identity. When auth is disabled (dev /
 * test), X-User-Id is honoured so curl-based smoke checks work.
 *
 * Public paths (`/api/openapi.json`, `/api/health`,
 * `/api/audit/verify`, `/api/audit/state`, `/api/bootstrap/...`)
 * bypass the auth check and tag the request as `bootstrap` or
 * `public`. The SVID route (`/api/identity/...`) is registered
 * AFTER this middleware and depends on the
 * `PROMPTSHEON_SVID_PUBLIC_KEY_PEM` env var to be set.
 */
export function authMiddleware(
  config: AppConfig,
  apiKeyRepo: ApiKeyRepo,
  opts: { svidPublicKeyPem?: string } = {},
) {
  const svidPublicKeyPem = opts.svidPublicKeyPem ?? process.env['PROMPTSHEON_SVID_PUBLIC_KEY_PEM'];
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
    if (typeof authHeader === 'string' && authHeader.length > 0) {
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
        void apiKeyRepo.updateLastUsed(apiKey.id);
        return;
      }

      if (authHeader.startsWith('SVID ')) {
        if (!svidPublicKeyPem) {
          return reply.code(503).send({
            error: {
              code: 'SVID_PUBLIC_KEY_NOT_CONFIGURED',
              message: 'PROMPTSHEON_SVID_PUBLIC_KEY_PEM is not set; SVID auth is unavailable',
            },
          });
        }
        const token = authHeader.slice(5);
        const v = verifySVID(token, svidPublicKeyPem);
        if (v === null) {
          return reply.code(401).send({
            error: { code: 'INVALID_SVID', message: 'SVID failed signature or freshness check' },
          });
        }
        (request as unknown as Record<string, string>).userId = v.payload.sub;
        (request as unknown as Record<string, string>).agentOrgId = v.payload.org;
        (request as unknown as Record<string, string>).agentClassification = v.payload.cls;
        (request as unknown as { orgContextBypass?: boolean }).orgContextBypass = true;
        (request as unknown as { principal?: unknown }).principal = {
          type: 'Agent',
          id: v.payload.sub,
          orgId: v.payload.org,
          classification: v.payload.cls,
        };
        return;
      }
    }

    return reply.code(401).send({ error: { code: 'UNAUTHORIZED', message: 'Missing authorization header' } });
  };
}