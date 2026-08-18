import type { FastifyInstance } from 'fastify';
import type { WorkspaceRepo } from '../repos/workspace.js';

export function registerWorkspaceRoutes(app: FastifyInstance, repo: WorkspaceRepo) {
  app.get('/api/workspaces', async (request, reply) => {
    const { page = 1, pageSize = 20 } = request.query as { page?: number; pageSize?: number };
    return reply.send(repo.findMany({ page, pageSize }));
  });

  app.get('/api/workspaces/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = repo.findById(id);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.post('/api/workspaces', async (request, reply) => {
    const data = request.body as { name: string; organization?: string };
    const item = repo.create(data);
    return reply.code(201).send(item);
  });

  app.put('/api/workspaces/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const data = request.body as { name?: string; organization?: string };
    const item = repo.update(id, data);
    return reply.send(item);
  });

  app.delete('/api/workspaces/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    repo.delete(id);
    return reply.code(204).send();
  });
}
