import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import {
  CreateProjectSchema,
  UpdateProjectSchema,
  PaginationSchema,
} from '@promptsheon/shared';
import type { ProjectRepo } from '../repos/project.js';
import { parseBody, parseQuery } from './validate.js';

const ListQuerySchema = PaginationSchema.extend({
  workspaceId: z.string().uuid().optional(),
});

export function registerProjectRoutes(app: FastifyInstance, repo: ProjectRepo) {
  app.get('/api/projects', async (request, reply) => {
    const parsed = parseQuery(reply, ListQuerySchema, request.query);
    if (!parsed.ok) return;
    const { workspaceId, page, pageSize } = parsed.data;
    if (workspaceId) return reply.send(repo.findByWorkspaceId(workspaceId));
    return reply.send(repo.findMany({ page, pageSize }));
  });

  app.get('/api/projects/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = repo.findById(id);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.post('/api/projects', async (request, reply) => {
    const parsed = parseBody(reply, CreateProjectSchema, request.body);
    if (!parsed.ok) return;
    const item = repo.create(parsed.data);
    return reply.code(201).send(item);
  });

  app.put('/api/projects/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, UpdateProjectSchema, request.body);
    if (!parsed.ok) return;
    const item = repo.update(id, parsed.data);
    return reply.send(item);
  });

  app.delete('/api/projects/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    repo.delete(id);
    return reply.code(204).send();
  });
}
