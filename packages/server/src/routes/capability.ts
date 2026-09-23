import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import {
  CreateCapabilitySchema,
  UpdateCapabilitySchema,
  PaginationSchema,
} from '@promptsheon/shared';
import type { CapabilityService } from '../application/capability-service.js';
import { parseBody, parseParams, parseQuery, sendNotFound } from './validate.js';

const IdParamsSchema = z.object({ id: z.string().uuid() });

const ListQuerySchema = PaginationSchema.extend({
  projectId: z.string().uuid().optional(),
});

function requireOrganization(request: unknown, reply: { code: (status: number) => { send: (body: unknown) => unknown } }): string | null {
  const context = (request as { agentOrgId?: string; orgContext?: { orgId?: string; organizationId?: string } } | undefined) ?? {};
  const organizationId = context.orgContext?.orgId ?? context.orgContext?.organizationId ?? context.agentOrgId;
  if (organizationId) return organizationId;
  void reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
  return null;
}

export function registerCapabilityRoutes(app: FastifyInstance, service: CapabilityService) {
  app.get('/api/capabilities', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsed = parseQuery(reply, ListQuerySchema, request.query);
    if (!parsed.ok) return;
    const { projectId, page, pageSize } = parsed.data;
    if (projectId) return reply.send(service.listByProject(projectId, organizationId));
    return reply.send(service.list(organizationId, { page, pageSize }));
  });

  app.get('/api/capabilities/:id', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsedParams = parseParams(reply, IdParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const item = service.get(parsedParams.data.id, organizationId);
    if (!item) return sendNotFound(reply, 'Capability', parsedParams.data.id);
    return reply.send(item);
  });

  app.post('/api/capabilities', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsed = parseBody(reply, CreateCapabilitySchema, request.body);
    if (!parsed.ok) return;
    const item = service.create(parsed.data, organizationId);
    if (!item) return sendNotFound(reply, 'Project', parsed.data.projectId);
    return reply.code(201).send(item);
  });

  app.put('/api/capabilities/:id', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsedParams = parseParams(reply, IdParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const parsed = parseBody(reply, UpdateCapabilitySchema, request.body);
    if (!parsed.ok) return;
    const item = service.update(parsedParams.data.id, organizationId, parsed.data);
    if (!item) return sendNotFound(reply, 'Capability', parsedParams.data.id);
    return reply.send(item);
  });

  app.delete('/api/capabilities/:id', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsed = parseParams(reply, IdParamsSchema, request.params);
    if (!parsed.ok) return;
    if (!service.remove(parsed.data.id, organizationId)) return sendNotFound(reply, 'Capability', parsed.data.id);
    return reply.code(204).send();
  });
}
