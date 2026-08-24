import type { FastifyRequest, FastifyReply } from 'fastify';
import { MembershipRepo } from '../repos/org.js';
import type { OrgRole } from '@promptsheon/shared';

declare module 'fastify' {
  interface FastifyRequest {
    orgContextBypass?: boolean;
  }
}

export interface OrgContext {
  userId: string;
  orgId: string;
  role: OrgRole;
}

/**
 * Extract org context from request headers.
 * Expected headers:
 *   X-User-Id: string (the user making the request)
 *   X-Org-Id: string (the org context)
 *
 * Verifies the user is a member of the org and attaches the role to
 * request.orgContext. Throws 403 if the user is not a member.
 *
 * Bypassed if request.orgContextBypass is set (for system routes).
 *
 * The system-actor bypass (\`X-User-Id: api\` with no X-Org-Id) was
 * previously always-on. Production deployments must opt in by setting
 * \`PROMPTSHEON_ALLOW_SYSTEM_ACTOR=true\`; otherwise the request is
 * rejected as MISSING_ORG_CONTEXT.
 */
export interface OrgContextOptions {
  /**
   * Allow the legacy system-actor bypass (`X-User-Id: api`, no
   * X-Org-Id) when the auth middleware installed that identity.
   * Defaults to true in non-production, false in production.
   */
  allowSystemActor?: boolean;
}

export function orgContextMiddleware(
  deps: { membershipRepo: MembershipRepo },
  opts: OrgContextOptions = {},
) {
  const env = process.env['PROMPTSHEON_NODE_ENV'] ?? process.env['NODE_ENV'] ?? 'development';
  const allowSystemActor =
    opts.allowSystemActor ??
    (env !== 'production' && process.env['PROMPTSHEON_ALLOW_SYSTEM_ACTOR'] !== 'false');

  return async (request: FastifyRequest, reply: FastifyReply) => {
    if (request.orgContextBypass) return;

    const userIdRaw = request.headers['x-user-id'];
    const orgIdRaw = request.headers['x-org-id'];
    const userId = Array.isArray(userIdRaw) ? userIdRaw[0] : userIdRaw;
    const orgId = Array.isArray(orgIdRaw) ? orgIdRaw[0] : orgIdRaw;

    // System actor — only honored if explicitly enabled.
    if (userId === 'api' && !orgId) {
      if (allowSystemActor) {
        (request as unknown as { orgContext: OrgContext }).orgContext = {
          userId: 'api',
          orgId: 'system',
          role: 'admin',
        };
        return;
      }
      return reply.code(401).send({
        error: {
          code: 'SYSTEM_ACTOR_DISABLED',
          message: 'system actor is not enabled in this environment',
        },
      });
    }

    if (!userId || !orgId) {
      return reply.code(401).send({ error: { code: 'MISSING_ORG_CONTEXT', message: 'X-User-Id and X-Org-Id headers required' } });
    }

    const members = deps.membershipRepo.findOrgMembers(orgId);
    const member = members.find((m) => m.userId === userId);
    if (!member) {
      return reply.code(403).send({ error: { code: 'NOT_ORG_MEMBER', message: 'User is not a member of the org' } });
    }

    (request as unknown as { orgContext: OrgContext }).orgContext = {
      userId,
      orgId,
      role: member.role,
    };
  };
}

/**
 * Role-check middleware. Use after orgContextMiddleware. Returns 403
 * if the user's org role is not in the allowed list.
 */
export function requireRole(allowedRoles: OrgRole[]) {
  return async (request: FastifyRequest, reply: FastifyReply) => {
    const ctx = (request as unknown as { orgContext?: OrgContext }).orgContext;
    if (!ctx) {
      return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'orgContextMiddleware must run first' } });
    }
    if (!allowedRoles.includes(ctx.role)) {
      return reply.code(403).send({ error: { code: 'INSUFFICIENT_ROLE', message: `Requires one of: ${allowedRoles.join(', ')}` } });
    }
  };
}

export function getOrgContext(request: FastifyRequest): OrgContext {
  const ctx = (request as unknown as { orgContext?: OrgContext }).orgContext;
  if (!ctx) throw new Error('orgContext not set on request');
  return ctx;
}