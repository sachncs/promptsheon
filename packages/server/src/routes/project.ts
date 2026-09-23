import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import {
  CreateProjectSchema,
  UpdateProjectSchema,
  PaginationSchema,
} from '@promptsheon/shared';
import type { ProjectService } from '../application/project-service.js';
import { parseBody, parseParams, parseQuery, sendNotFound } from './validate.js';

const IdParamsSchema = z.object({ id: z.string().uuid() });

const ListQuerySchema = PaginationSchema.extend({
  workspaceId: z.string().uuid().optional(),
});

function requireOrganization(request: unknown, reply: { code: (status: number) => { send: (body: unknown) => unknown } }): string | null {
  const context = (request as { agentOrgId?: string; orgContext?: { orgId?: string; organizationId?: string } } | undefined) ?? {};
  const organizationId = context.orgContext?.orgId ?? context.orgContext?.organizationId ?? context.agentOrgId;
  if (organizationId) return organizationId;
  void reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
  return null;
}

export function registerProjectRoutes(app: FastifyInstance, service: ProjectService) {
  app.get('/api/projects', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsed = parseQuery(reply, ListQuerySchema, request.query);
    if (!parsed.ok) return;
    const { workspaceId, page, pageSize } = parsed.data;
    if (workspaceId) return reply.send(service.listByWorkspace(workspaceId, organizationId));
    return reply.send(service.list(organizationId, { page, pageSize }));
  });

  app.get('/api/projects/:id', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsed = parseParams(reply, IdParamsSchema, request.params);
    if (!parsed.ok) return;
    const item = service.get(parsed.data.id, organizationId);
    if (!item) return sendNotFound(reply, 'Project', parsed.data.id);
    return reply.send(item);
  });

  app.post('/api/projects', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsed = parseBody(reply, CreateProjectSchema, request.body);
    if (!parsed.ok) return;
    const item = service.create(parsed.data, organizationId);
    if (!item) return sendNotFound(reply, 'Workspace', parsed.data.workspaceId);
    return reply.code(201).send(item);
  });

  app.put('/api/projects/:id', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsedParams = parseParams(reply, IdParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const parsed = parseBody(reply, UpdateProjectSchema, request.body);
    if (!parsed.ok) return;
    const item = service.update(parsedParams.data.id, organizationId, parsed.data);
    if (!item) return sendNotFound(reply, 'Project', parsedParams.data.id);
    return reply.send(item);
  });

  app.delete('/api/projects/:id', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsed = parseParams(reply, IdParamsSchema, request.params);
    if (!parsed.ok) return;
    if (!service.remove(parsed.data.id, organizationId)) return sendNotFound(reply, 'Project', parsed.data.id);
    return reply.code(204).send();
  });
}
