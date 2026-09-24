import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type { UserRepo } from '../repos/user.js';
import { parseBody, parseParams } from './validate.js';
import { AuditChain } from '../audit/chain.js';
import { requireAdmin } from '../middleware/admin.js';
import type { MembershipRepo } from '../repos/org.js';

const CreateUserSchema = z.object({
  email: z.string().email().max(255),
  name: z.string().min(1).max(255),
  role: z.enum(['admin', 'editor', 'reader', 'system']).optional(),
});

const UpdateRoleSchema = z.object({
  role: z.enum(['admin', 'editor', 'reader', 'system']),
});

const UserParamsSchema = z.object({
  id: z.string().trim().min(1).max(255),
});

interface RequestUserContext {
  userId?: string;
  agentOrgId?: string;
  orgContext?: { orgId?: string; organizationId?: string };
}

function requireOrganization(request: unknown, reply: { code: (status: number) => { send: (body: unknown) => unknown } }): string | null {
  const context = (request as RequestUserContext | undefined) ?? {};
  const organizationId = context.orgContext?.orgId ?? context.orgContext?.organizationId ?? context.agentOrgId;
  if (organizationId) return organizationId;
  void reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
  return null;
}

function actorOf(request: unknown): string {
  const ctx = (request as RequestUserContext | undefined) ?? {};
  return ctx.userId ?? 'system';
}

export function registerUserRoutes(
  app: FastifyInstance,
  deps: { userRepo: UserRepo; auditChain: AuditChain; membershipRepo?: MembershipRepo },
) {
  app.get('/api/users', { preHandler: requireAdmin() }, async (request, reply) => {
    const orgId = requireOrganization(request, reply);
    if (!orgId) return;
    const users = deps.userRepo.listForOrg(orgId);
    return reply.send({ users });
  });

  app.get('/api/users/me', async (request, reply) => {
    const orgId = requireOrganization(request, reply);
    if (!orgId) return;
    const userId = actorOf(request);
    const user = deps.userRepo.findByIdInOrg(userId, orgId);
    if (!user) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Current user not found' } });
    }
    return reply.send(user);
  });

  app.get('/api/users/:id', async (request, reply) => {
    const orgId = requireOrganization(request, reply);
    if (!orgId) return;
    const parsedParams = parseParams(reply, UserParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const user = deps.userRepo.findByIdInOrg(id, orgId);
    if (!user) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'User not found' } });
    }
    return reply.send(user);
  });

  app.post('/api/users', { preHandler: requireAdmin() }, async (request, reply) => {
    const orgId = requireOrganization(request, reply);
    if (!orgId) return;
    const parsed = parseBody(reply, CreateUserSchema, request.body);
    if (!parsed.ok) return;
    const user = deps.userRepo.createInOrg(parsed.data, orgId);
    if (deps.membershipRepo) {
      const role = parsed.data.role === 'admin'
        ? 'admin'
        : parsed.data.role === 'editor'
          ? 'editor'
          : 'viewer';
      deps.membershipRepo.addOrgMember(orgId, user.id, role);
    }
    deps.auditChain.append({
      userId: actorOf(request),
      action: 'user.create',
      resource: 'user',
      details: JSON.stringify({ userId: user.id, email: user.email, role: user.role }),
      resourceKind: 'user',
      resourceId: user.id,
    });
    return reply.code(201).send(user);
  });

  app.put('/api/users/:id/role', { preHandler: requireAdmin() }, async (request, reply) => {
    const orgId = requireOrganization(request, reply);
    const parsedParams = parseParams(reply, UserParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const parsed = parseBody(reply, UpdateRoleSchema, request.body);
    if (!parsed.ok) return;
    if (!orgId) return;
    const existing = deps.userRepo.findByIdInOrg(id, orgId);
    if (!existing) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'User not found' } });
    }
    const user = deps.userRepo.updateRoleInOrg(id, orgId, parsed.data.role);
    if (!user) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'User not found' } });
    }
    deps.auditChain.append({
      userId: actorOf(request),
      action: 'user.update_role',
      resource: 'user',
      details: JSON.stringify({ userId: id, newRole: parsed.data.role }),
      resourceKind: 'user',
      resourceId: id,
    });
    return reply.send(user);
  });
}
