import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type { UserRepo } from '../repos/user.js';
import { parseBody } from './validate.js';
import { AuditChain } from '../audit/chain.js';

const CreateUserSchema = z.object({
  email: z.string().email().max(255),
  name: z.string().min(1).max(255),
  role: z.enum(['admin', 'editor', 'reader', 'system']).optional(),
});

const UpdateRoleSchema = z.object({
  role: z.enum(['admin', 'editor', 'reader', 'system']),
});

interface RequestUserContext {
  userId?: string;
}

function actorOf(request: unknown): string {
  const ctx = (request as RequestUserContext | undefined) ?? {};
  return ctx.userId ?? 'system';
}

export function registerUserRoutes(
  app: FastifyInstance,
  deps: { userRepo: UserRepo; auditChain: AuditChain },
) {
  app.get('/api/users', async (_request, reply) => {
    return reply.send({ users: deps.userRepo.list() });
  });

  app.get('/api/users/me', async (request, reply) => {
    const userId = actorOf(request);
    const user = deps.userRepo.findById(userId);
    if (!user) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Current user not found' } });
    }
    return reply.send(user);
  });

  app.get('/api/users/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const user = deps.userRepo.findById(id);
    if (!user) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'User not found' } });
    }
    return reply.send(user);
  });

  app.post('/api/users', async (request, reply) => {
    const parsed = parseBody(reply, CreateUserSchema, request.body);
    if (!parsed.ok) return;
    const user = deps.userRepo.create(parsed.data);
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

  app.put('/api/users/:id/role', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, UpdateRoleSchema, request.body);
    if (!parsed.ok) return;
    const user = deps.userRepo.updateRole(id, parsed.data.role);
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