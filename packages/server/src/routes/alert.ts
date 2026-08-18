import type { FastifyInstance } from 'fastify';
import type { AlertRepo } from '../repos/alert.js';

export function registerAlertRoutes(app: FastifyInstance, repo: AlertRepo) {
  app.get('/api/alert-rules', async (_request, reply) => {
    return reply.send(repo.findRules());
  });

  app.get('/api/alert-rules/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = repo.findRuleById(id);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.post('/api/alert-rules', async (request, reply) => {
    const data = request.body as { name: string; type: string; severity: string; enabled?: boolean; threshold?: number; duration?: number; window?: number; config?: string };
    const item = repo.createRule(data);
    return reply.code(201).send(item);
  });

  app.put('/api/alert-rules/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const data = request.body as { name?: string; threshold?: number; enabled?: boolean };
    const item = repo.updateRule(id, data);
    return reply.send(item);
  });

  app.delete('/api/alert-rules/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    repo.deleteRule(id);
    return reply.code(204).send();
  });

  app.get('/api/alerts', async (request, reply) => {
    const { status } = request.query as { status?: string };
    return reply.send(repo.findAlerts(status));
  });

  app.put('/api/alerts/:id/acknowledge', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = repo.updateAlert(id, { acknowledgedAt: new Date().toISOString() });
    return reply.send(item);
  });
}
