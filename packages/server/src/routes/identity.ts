import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { cedarGate } from '../policy/gate.js';
import { NotFoundError } from '@promptsheon/shared';
import type { IdentityService } from '../application/identity-service.js';
import { parseParams } from './validate.js';

const MintKeySchema = z.object({
  agentId: z.string().min(1).max(128),
  organizationId: z.string().min(1).max(128),
  ttlDays: z.number().int().min(1).max(365).optional().default(30),
  scope: z.string().max(2048).optional(),
});

const MintSvidSchema = z.object({
  agentId: z.string().min(1).max(128),
  organizationId: z.string().min(1).max(128),
  signingKeyPem: z.string().min(1), // PKCS8 PEM
  ttlSeconds: z.number().int().min(60).max(86_400).optional().default(900),
  scope: z.array(z.string()).optional(),
  classification: z.string().min(1).max(64).optional(),
});

const IdentityParamsSchema = z.object({
  id: z.string().trim().min(1).max(255),
});

export interface IdentityDeps {
  service: IdentityService;
}

function requireOrganization(
  request: { orgContext?: { orgId?: string }; agentOrgId?: string },
  reply: { code: (status: number) => { send: (body: unknown) => unknown } },
): string | null {
  const organizationId = request.orgContext?.orgId ?? request.agentOrgId;
  if (!organizationId) {
    void reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'organization context is required' } });
    return null;
  }
  return organizationId;
}

/**
 * Register the agent-identity routes. Three endpoints:
 *
 *  - POST /api/identity/keys   — mint a long-lived apikey
 *  - POST /api/identity/svid   — mint a short-lived SVID
 *  - DELETE /api/identity/:id  — revoke (sets revoked_at)
 *
 * All three are Cedar-gated. The Cedar policy in
 * `policies/promptsheon.cedar` covers them via:
 *   - Identity::Mint (apikey + SVID)
 *   - Identity::Revoke
 * The gates install the action name on the request and the
 * policy decides whether the principal can mint / revoke.
 */
export function registerIdentityRoutes(app: FastifyInstance, deps: IdentityDeps): void {
  const adminOnly = cedarGate({ action: 'Identity::Mint', resource: 'default' });
  const adminOrApproverRevoke = cedarGate({
    action: 'Identity::Revoke',
    resource: 'default',
  });

  app.post(
    '/api/identity/keys',
    { preHandler: adminOnly },
    async (request, reply) => {
      const parsed = MintKeySchema.safeParse(request.body);
      if (!parsed.success) {
        return reply.code(422).send({
          error: {
            code: 'VALIDATION_ERROR',
            message: 'invalid mint-key body',
            issues: parsed.error.issues,
          },
        });
      }
      const organizationId = requireOrganization(request, reply);
      if (!organizationId) return;
      if (parsed.data.organizationId !== organizationId) {
        return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'organization not found' } });
      }
      const material = deps.service.mintApiKey({
        agentId: parsed.data.agentId,
        organizationId: parsed.data.organizationId,
        ttlDays: parsed.data.ttlDays,
        scope: parsed.data.scope,
      });
      return reply.code(201).send({
        id: material.id,
        token: material.token,
        hash: material.hash,
        agentId: material.agentId,
        organizationId: material.orgId,
        scope: material.scope,
        issuedAt: material.issuedAt,
        expiresAt: material.expiresAt,
      });
    },
  );

  app.post(
    '/api/identity/svid',
    { preHandler: adminOnly },
    async (request, reply) => {
      const parsed = MintSvidSchema.safeParse(request.body);
      if (!parsed.success) {
        return reply.code(422).send({
          error: {
            code: 'VALIDATION_ERROR',
            message: 'invalid mint-svid body',
            issues: parsed.error.issues,
          },
        });
      }
      const organizationId = requireOrganization(request, reply);
      if (!organizationId) return;
      if (parsed.data.organizationId !== organizationId) {
        return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'organization not found' } });
      }
      try {
        const material = deps.service.mintSvid({
          agentId: parsed.data.agentId,
          organizationId: parsed.data.organizationId,
          signingKeyPem: parsed.data.signingKeyPem,
          ttlSeconds: parsed.data.ttlSeconds,
          scope: parsed.data.scope,
          classification: parsed.data.classification,
        });
        return reply.code(201).send(material);
      } catch (err) {
        return reply.code(400).send({
          error: { code: 'INVALID_SIGNING_KEY', message: (err as Error).message },
        });
      }
    },
  );

  app.delete(
    '/api/identity/:id',
    { preHandler: adminOrApproverRevoke },
    async (request, reply) => {
      const parsedParams = parseParams(reply, IdentityParamsSchema, request.params);
      if (!parsedParams.ok) return;
      const { id } = parsedParams.data;
      const organizationId = requireOrganization(request, reply);
      if (!organizationId) return;
      if (!deps.service.revoke(id, organizationId)) {
        return reply.code(404).send({
          error: { code: 'IDENTITY_NOT_FOUND', message: `identity ${id} not found` },
        });
      }
      return reply.code(204).send();
    },
  );
}
