import type { FastifyInstance, FastifyReply, FastifyRequest } from 'fastify';
import { z } from 'zod';
import {
  CreateWorkspaceSchema,
  UpdateWorkspaceSchema,
  PaginationSchema,
} from '@promptsheon/shared';
import type { WorkspaceService } from '../application/workspace-service.js';
import { parseBody, parseParams, parseQuery, sendNotFound } from './validate.js';

const IdParamsSchema = z.object({ id: z.string().uuid() });

function requireOrganization(request: FastifyRequest, reply: FastifyReply): string | null {
  const organizationId = request.orgContext?.orgId ?? request.agentOrgId;
  if (organizationId) return organizationId;
  void reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
  return null;
}

export function registerWorkspaceRoutes(app: FastifyInstance, service: WorkspaceService) {
  app.get('/api/workspaces', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsed = parseQuery(reply, PaginationSchema, request.query);
    if (!parsed.ok) return;
    return reply.send(service.list(organizationId, parsed.data));
  });

  app.get('/api/workspaces/:id', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsed = parseParams(reply, IdParamsSchema, request.params);
    if (!parsed.ok) return;
    const item = service.get(parsed.data.id, organizationId);
    if (!item) return sendNotFound(reply, 'Workspace', parsed.data.id);
    return reply.send(item);
  });

  app.post('/api/workspaces', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsed = parseBody(reply, CreateWorkspaceSchema, request.body);
    if (!parsed.ok) return;
    const item = service.create(parsed.data, organizationId);
    if (!item) return sendNotFound(reply, 'Workspace', 'new');
    return reply.code(201).send(item);
  });

  app.put('/api/workspaces/:id', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsedParams = parseParams(reply, IdParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const parsed = parseBody(reply, UpdateWorkspaceSchema, request.body);
    if (!parsed.ok) return;
    const item = service.update(parsedParams.data.id, organizationId, parsed.data);
    if (!item) return sendNotFound(reply, 'Workspace', parsedParams.data.id);
    return reply.send(item);
  });

  app.delete('/api/workspaces/:id', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsed = parseParams(reply, IdParamsSchema, request.params);
    if (!parsed.ok) return;
    if (!service.remove(parsed.data.id, organizationId)) return sendNotFound(reply, 'Workspace', parsed.data.id);
    return reply.code(204).send();
  });
}
