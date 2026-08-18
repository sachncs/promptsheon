import type { FastifyInstance } from 'fastify';
import type { CapabilityRepo } from '../repos/capability.js';

export function registerCapabilityRoutes(app: FastifyInstance, repo: CapabilityRepo) {
  app.get('/api/capabilities', async (request, reply) => {
    const { projectId, page = 1, pageSize = 20 } = request.query as { projectId?: string; page?: number; pageSize?: number };
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
    const data = request.body as { projectId: string; name: string; description?: string };
    const item = repo.create(data);
    return reply.code(201).send(item);
  });

  app.put('/api/capabilities/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const data = request.body as { name?: string; description?: string };
    const item = repo.update(id, data);
    return reply.send(item);
  });

  app.delete('/api/capabilities/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    repo.delete(id);
    return reply.code(204).send();
  });
}
