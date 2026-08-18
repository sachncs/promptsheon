import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { PaginationSchema } from '@promptsheon/shared';
import type { VersionRepo } from '../repos/version.js';
import { parseBody, parseQuery } from './validate.js';

const ListQuerySchema = PaginationSchema.extend({
  capabilityId: z.string().uuid().optional(),
});

const CreateVersionSchema = z.object({
  capabilityId: z.string().uuid(),
  version: z.number().int().positive(),
  manifest: z.string().min(1),
  manifestHash: z.string().min(1),
  createdBy: z.string().optional(),
});

export function registerVersionRoutes(app: FastifyInstance, repo: VersionRepo) {
  app.get('/api/capability-versions', async (request, reply) => {
    const parsed = parseQuery(reply, ListQuerySchema, request.query);
    if (!parsed.ok) return;
    const { capabilityId, page, pageSize } = parsed.data;
    if (capabilityId) return reply.send(repo.findByCapabilityId(capabilityId));
    return reply.send(repo.findMany({ page, pageSize }));
  });

  app.get('/api/capability-versions/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = repo.findById(id);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.post('/api/capability-versions', async (request, reply) => {
    const parsed = parseBody(reply, CreateVersionSchema, request.body);
    if (!parsed.ok) return;
    const item = repo.create(parsed.data);
    return reply.code(201).send(item);
  });

  app.delete('/api/capability-versions/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    repo.delete(id);
    return reply.code(204).send();
  });
}
