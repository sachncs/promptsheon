import type { FastifyInstance } from 'fastify';
import type { ProjectRepo } from '../repos/project.js';

export function registerProjectRoutes(app: FastifyInstance, repo: ProjectRepo) {
  app.get('/api/projects', async (request, reply) => {
    const { workspaceId, page = 1, pageSize = 20 } = request.query as { workspaceId?: string; page?: number; pageSize?: number };
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
    const data = request.body as { workspaceId: string; name: string; description?: string };
    const item = repo.create(data);
    return reply.code(201).send(item);
  });

  app.put('/api/projects/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const data = request.body as { name?: string; description?: string };
    const item = repo.update(id, data);
    return reply.send(item);
  });

  app.delete('/api/projects/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    repo.delete(id);
    return reply.code(204).send();
  });
}
