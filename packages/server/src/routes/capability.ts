import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import {
  CreateCapabilitySchema,
  UpdateCapabilitySchema,
  PaginationSchema,
} from '@promptsheon/shared';
import type { CapabilityRepo } from '../repos/capability.js';
import { parseBody, parseQuery } from './validate.js';

const ListQuerySchema = PaginationSchema.extend({
  projectId: z.string().uuid().optional(),
});

export function registerCapabilityRoutes(app: FastifyInstance, repo: CapabilityRepo) {
  app.get('/api/capabilities', async (request, reply) => {
    const parsed = parseQuery(reply, ListQuerySchema, request.query);
    if (!parsed.ok) return;
    const { projectId, page, pageSize } = parsed.data;
    if (projectId) return reply.send(repo.findByProjectId(projectId));
    return reply.send(repo.findMany({ page, pageSize }));
  });

  app.get('/api/capabilities/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = repo.findById(id);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.post('/api/capabilities', async (request, reply) => {
    const parsed = parseBody(reply, CreateCapabilitySchema, request.body);
    if (!parsed.ok) return;
    const item = repo.create(parsed.data);
    return reply.code(201).send(item);
  });

  app.put('/api/capabilities/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, UpdateCapabilitySchema, request.body);
    if (!parsed.ok) return;
    const item = repo.update(id, parsed.data);
    return reply.send(item);
  });

  app.delete('/api/capabilities/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    repo.delete(id);
    return reply.code(204).send();
  });
}
