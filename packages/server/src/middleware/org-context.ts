import type { FastifyRequest, FastifyReply } from 'fastify';
import { MembershipRepo } from '../repos/org.js';
import type { OrgRole } from '@promptsheon/shared';

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
 */
export function orgContextMiddleware(deps: { membershipRepo: MembershipRepo }) {
  return async (request: FastifyRequest, reply: FastifyReply) => {
    if ((request as unknown as { orgContextBypass?: boolean }).orgContextBypass) return;

    const userIdRaw = request.headers['x-user-id'];
    const orgIdRaw = request.headers['x-org-id'];
    const userId = Array.isArray(userIdRaw) ? userIdRaw[0] : userIdRaw;
    const orgId = Array.isArray(orgIdRaw) ? orgIdRaw[0] : orgIdRaw;
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