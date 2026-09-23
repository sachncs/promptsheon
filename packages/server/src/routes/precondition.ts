import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { CreatePreconditionSchema } from '@promptsheon/shared';
import type { PreconditionRepo } from '../repos/precondition.js';
import { parseBody, parseQuery } from './validate.js';

const ListPreconditionsQuerySchema = z.object({
  capabilityId: z.string().min(1).optional(),
});

const UpdatePreconditionSchema = z.object({
  name: z.string().min(1).max(255).optional(),
  command: z.string().min(1).optional(),
  timeoutSec: z.number().int().min(1).max(3600).optional(),
  enabled: z.boolean().optional(),
});

interface RequestOrganizationContext {
  agentOrgId?: string;
  orgContext?: { orgId?: string; organizationId?: string };
}

function requireOrganization(request: unknown, reply: { code: (status: number) => { send: (body: unknown) => unknown } }): string | null {
  const context = (request as RequestOrganizationContext | undefined) ?? {};
  const organizationId = context.orgContext?.orgId ?? context.orgContext?.organizationId ?? context.agentOrgId;
  if (organizationId) return organizationId;
  void reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
  return null;
}

export function registerPreconditionRoutes(app: FastifyInstance, repo: PreconditionRepo) {
  app.get('/api/preconditions', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsed = parseQuery(reply, ListPreconditionsQuerySchema, request.query);
    if (!parsed.ok) return;
    const { capabilityId } = parsed.data;
    if (capabilityId) return reply.send(repo.findByCapabilityIdInOrg(capabilityId, organizationId));
    return reply.send([]);
  });

  app.get('/api/preconditions/:id', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const { id } = request.params as { id: string };
    const item = repo.findByIdInOrg(id, organizationId);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.post('/api/preconditions', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsed = parseBody(reply, CreatePreconditionSchema, request.body);
    if (!parsed.ok) return;
    const item = repo.createInOrg(parsed.data, organizationId);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'capability not found' } });
    return reply.code(201).send(item);
  });

  app.put('/api/preconditions/:id', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, UpdatePreconditionSchema, request.body);
    if (!parsed.ok) return;
    const updated = repo.updateInOrg(id, organizationId, parsed.data);
    if (!updated) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'precondition not found' } });
    return reply.send(updated);
  });

  app.delete('/api/preconditions/:id', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const { id } = request.params as { id: string };
    repo.deleteInOrg(id, organizationId);
    return reply.code(204).send();
  });
}
