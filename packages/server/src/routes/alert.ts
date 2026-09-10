import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import {
  CreateAlertRuleSchema,
  UpdateAlertRuleSchema,
} from '@promptsheon/shared';
import type { AlertRepo } from '../repos/alert.js';
import { parseBody, parseQuery } from './validate.js';

const ListAlertsQuerySchema = z.object({
  status: z.string().optional(),
});

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
    const parsed = parseBody(reply, CreateAlertRuleSchema, request.body);
    if (!parsed.ok) return;
    const { config, ...rest } = parsed.data;
    const item = repo.createRule({ ...rest, config: config ? JSON.stringify(config) : undefined });
    return reply.code(201).send(item);
  });

  app.put('/api/alert-rules/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, UpdateAlertRuleSchema, request.body);
    if (!parsed.ok) return;
    const { config, ...rest } = parsed.data;
    const item = repo.updateRule(id, { ...rest, config: config ? JSON.stringify(config) : undefined });
    return reply.send(item);
  });

  app.delete('/api/alert-rules/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    repo.deleteRule(id);
    return reply.code(204).send();
  });

  app.get('/api/alerts', async (request, reply) => {
    const parsed = parseQuery(reply, ListAlertsQuerySchema, request.query);
    if (!parsed.ok) return;
    return reply.send(repo.findAlerts(parsed.data.status));
  });

  app.put('/api/alerts/:id/acknowledge', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = repo.updateAlert(id, { acknowledgedAt: new Date().toISOString() });
    return reply.send(item);
  });
}
