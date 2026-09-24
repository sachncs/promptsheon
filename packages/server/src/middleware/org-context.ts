import type { FastifyRequest, FastifyReply } from 'fastify';
import { MembershipRepo } from '../repos/org.js';
import type { OrgRole } from '@promptsheon/shared';

declare module 'fastify' {
  interface FastifyRequest {
    orgContextBypass?: boolean;
    orgContext?: OrgContext;
  }
}

export interface OrgContext {
  userId: string;
  orgId: string;
  role: OrgRole;
}

/**
 * Extract organization context from the authenticated principal.
 *
 * Verifies the principal's organization membership and attaches the
 * role to request.orgContext. Tenant headers are deliberately ignored;
 * organization scope is established only by authentication.
 *
 * Bypassed if request.orgContextBypass is set (for system routes).
 *
 */
export function orgContextMiddleware(
  deps: { membershipRepo: MembershipRepo },
) {
  return async (request: FastifyRequest, reply: FastifyReply) => {
    if (request.orgContextBypass) return;

    const principal = request.principal;
    if (!principal || principal.type === 'Agent' || principal.type === 'System') {
      return reply.code(401).send({ error: { code: 'MISSING_ORG_CONTEXT', message: 'authenticated organization context required' } });
    }
    const { id: userId, orgId } = principal;

    const members = deps.membershipRepo.findOrgMembers(orgId);
    const member = members.find((m) => m.userId === userId);
    if (!member) {
      return reply.code(403).send({ error: { code: 'NOT_ORG_MEMBER', message: 'User is not a member of the org' } });
    }

    request.orgContext = {
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
    const ctx = request.orgContext;
    if (!ctx) {
      return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'orgContextMiddleware must run first' } });
    }
    if (!allowedRoles.includes(ctx.role)) {
      return reply.code(403).send({ error: { code: 'INSUFFICIENT_ROLE', message: `Requires one of: ${allowedRoles.join(', ')}` } });
    }
  };
}

export function getOrgContext(request: FastifyRequest): OrgContext {
  const ctx = request.orgContext;
  if (!ctx) throw new Error('orgContext not set on request');
  return ctx;
}

/**
 * Bind a route's organization identifier to the authenticated context.
 * Requests without an authenticated organization remain out of scope.
 */
export function assertOrgScope(
  request: FastifyRequest,
  requestedOrgId: string,
  reply: FastifyReply,
): boolean {
  const activeOrgId = request.orgContext?.orgId ?? request.agentOrgId;
  if (activeOrgId && activeOrgId !== requestedOrgId) {
    void reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'organization not found' } });
    return false;
  }
  return true;
}
