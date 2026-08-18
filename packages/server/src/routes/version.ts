import type { FastifyInstance } from 'fastify';
import type { VersionRepo } from '../repos/version.js';

export function registerVersionRoutes(app: FastifyInstance, repo: VersionRepo) {
  app.get('/api/capability-versions', async (request, reply) => {
    const { capabilityId, page = 1, pageSize = 20 } = request.query as { capabilityId?: string; page?: number; pageSize?: number };
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
    const data = request.body as { capabilityId: string; version: number; manifest: string; manifestHash: string; createdBy?: string };
    const item = repo.create(data);
    return reply.code(201).send(item);
  });

  app.delete('/api/capability-versions/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    repo.delete(id);
    return reply.code(204).send();
  });
}
