import type { FastifyInstance } from 'fastify';
import type { PreconditionRepo } from '../repos/precondition.js';

export function registerPreconditionRoutes(app: FastifyInstance, repo: PreconditionRepo) {
  app.get('/api/preconditions', async (request, reply) => {
    const { capabilityId } = request.query as { capabilityId?: string };
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
    const data = request.body as { capabilityId: string; name: string; command: string; timeoutSec?: number; enabled?: boolean };
    const item = repo.create(data);
    return reply.code(201).send(item);
  });

  app.delete('/api/preconditions/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    repo.delete(id);
    return reply.code(204).send();
  });
}
