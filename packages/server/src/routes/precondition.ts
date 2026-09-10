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

export function registerPreconditionRoutes(app: FastifyInstance, repo: PreconditionRepo) {
  app.get('/api/preconditions', async (request, reply) => {
    const parsed = parseQuery(reply, ListPreconditionsQuerySchema, request.query);
    if (!parsed.ok) return;
    const { capabilityId } = parsed.data;
    if (capabilityId) return reply.send(repo.findByCapabilityId(capabilityId));
    return reply.send([]);
  });

  app.get('/api/preconditions/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = repo.findById(id);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.post('/api/preconditions', async (request, reply) => {
    const parsed = parseBody(reply, CreatePreconditionSchema, request.body);
    if (!parsed.ok) return;
    const item = repo.create(parsed.data);
    return reply.code(201).send(item);
  });

  app.put('/api/preconditions/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, UpdatePreconditionSchema, request.body);
    if (!parsed.ok) return;
    const updated = repo.update(id, parsed.data);
    if (!updated) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'precondition not found' } });
    return reply.send(updated);
  });

  app.delete('/api/preconditions/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    repo.delete(id);
    return reply.code(204).send();
  });
}