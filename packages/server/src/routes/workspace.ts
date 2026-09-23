import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import {
  CreateWorkspaceSchema,
  UpdateWorkspaceSchema,
  PaginationSchema,
} from '@promptsheon/shared';
import type { WorkspaceService } from '../application/workspace-service.js';
import { parseBody, parseParams, parseQuery, sendNotFound } from './validate.js';

const IdParamsSchema = z.object({ id: z.string().uuid() });

export function registerWorkspaceRoutes(app: FastifyInstance, service: WorkspaceService) {
  app.get('/api/workspaces', async (request, reply) => {
    const parsed = parseQuery(reply, PaginationSchema, request.query);
    if (!parsed.ok) return;
    return reply.send(service.list(parsed.data));
  });

  app.get('/api/workspaces/:id', async (request, reply) => {
    const parsed = parseParams(reply, IdParamsSchema, request.params);
    if (!parsed.ok) return;
    const item = service.get(parsed.data.id);
    if (!item) return sendNotFound(reply, 'Workspace', parsed.data.id);
    return reply.send(item);
  });

  app.post('/api/workspaces', async (request, reply) => {
    const parsed = parseBody(reply, CreateWorkspaceSchema, request.body);
    if (!parsed.ok) return;
    const item = service.create(parsed.data);
    return reply.code(201).send(item);
  });

  app.put('/api/workspaces/:id', async (request, reply) => {
    const parsedParams = parseParams(reply, IdParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const parsed = parseBody(reply, UpdateWorkspaceSchema, request.body);
    if (!parsed.ok) return;
    const item = service.update(parsedParams.data.id, parsed.data);
    if (!item) return sendNotFound(reply, 'Workspace', parsedParams.data.id);
    return reply.send(item);
  });

  app.delete('/api/workspaces/:id', async (request, reply) => {
    const parsed = parseParams(reply, IdParamsSchema, request.params);
    if (!parsed.ok) return;
    if (!service.remove(parsed.data.id)) return sendNotFound(reply, 'Workspace', parsed.data.id);
    return reply.code(204).send();
  });
}
