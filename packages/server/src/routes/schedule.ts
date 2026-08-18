import type { FastifyInstance } from 'fastify';
import type { ScheduleRepo } from '../repos/schedule.js';

export function registerScheduleRoutes(app: FastifyInstance, repo: ScheduleRepo) {
  app.get('/api/schedules', async (request, reply) => {
    const { page = 1, pageSize = 20 } = request.query as { page?: number; pageSize?: number };
    return reply.send(repo.findMany({ page, pageSize }));
  });

  app.get('/api/schedules/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = repo.findById(id);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.post('/api/schedules', async (request, reply) => {
    const data = request.body as { workspaceId: string; releaseId: string; kind: string; cron: string; enabled?: boolean };
    const item = repo.create(data);
    return reply.code(201).send(item);
  });

  app.put('/api/schedules/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const data = request.body as { cron?: string; enabled?: boolean; nextFireAt?: string };
    const item = repo.update(id, data);
    return reply.send(item);
  });

  app.delete('/api/schedules/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    repo.delete(id);
    return reply.code(204).send();
  });
}
