import type { FastifyInstance } from 'fastify';
import {
  CreateWorkspaceSchema,
  UpdateWorkspaceSchema,
  PaginationSchema,
} from '@promptsheon/shared';
import type { WorkspaceRepo } from '../repos/workspace.js';
import { parseBody, parseQuery } from './validate.js';

export function registerWorkspaceRoutes(app: FastifyInstance, repo: WorkspaceRepo) {
  app.get('/api/workspaces', async (request, reply) => {
    const parsed = parseQuery(reply, PaginationSchema, request.query);
    if (!parsed.ok) return;
    return reply.send(repo.findMany(parsed.data));
  });

  app.get('/api/workspaces/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = repo.findById(id);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.post('/api/workspaces', async (request, reply) => {
    const parsed = parseBody(reply, CreateWorkspaceSchema, request.body);
    if (!parsed.ok) return;
    const item = repo.create(parsed.data);
    return reply.code(201).send(item);
  });

  app.put('/api/workspaces/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, UpdateWorkspaceSchema, request.body);
    if (!parsed.ok) return;
    const item = repo.update(id, parsed.data);
    return reply.send(item);
  });

  app.delete('/api/workspaces/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    repo.delete(id);
    return reply.code(204).send();
  });
}
