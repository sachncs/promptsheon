import type { FastifyInstance } from 'fastify';
import type { ReleaseRepo } from '../repos/release.js';

export function registerReleaseRoutes(app: FastifyInstance, repo: ReleaseRepo) {
  app.get('/api/releases', async (request, reply) => {
    const { capabilityId, page = 1, pageSize = 20 } = request.query as { capabilityId?: string; page?: number; pageSize?: number };
    if (capabilityId) return reply.send(repo.findByCapabilityId(capabilityId));
    return reply.send(repo.findMany({ page, pageSize }));
  });

  app.get('/api/releases/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = repo.findById(id);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.post('/api/releases', async (request, reply) => {
    const data = request.body as {
      capabilityId: string;
      capabilityVersion: number;
      capabilityVersionId: string | null;
      manifest: string;
      environment: string;
      createdBy?: string;
      canaryPercent?: number;
    };
    const item = repo.create(data);
    return reply.code(201).send(item);
  });

  app.put('/api/releases/:id/activate', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = repo.updateStatus(id, 'active');
    return reply.send(item);
  });

  app.put('/api/releases/:id/supersede', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = repo.updateStatus(id, 'superseded');
    return reply.send(item);
  });
}
