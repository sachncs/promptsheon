import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { parseBody } from './validate.js';
import type { RetentionSweeper } from '../scheduler/retention-sweeper.js';

const RetentionSchema = z.object({
  organizationId: z.string(),
  days: z.number().int().min(1).max(3650),
});

export interface RetentionRouteDeps {
  sweeper: RetentionSweeper;
  adminOnly: (request: unknown) => boolean;
}

export function registerRetentionRoutes(app: FastifyInstance, deps: RetentionRouteDeps): void {
  app.get('/api/orgs/:id/retention', async (request, reply) => {
    const { id } = request.params as { id: string };
    if (!deps.adminOnly(request)) {
      return reply.code(403).send({ error: { code: 'FORBIDDEN', message: 'admin only' } });
    }
    return reply.send({
      organizationId: id,
      retentionDays: deps.sweeper.retentionDaysFor(id),
    });
  });

  app.put('/api/orgs/:id/retention', async (request, reply) => {
    const { id } = request.params as { id: string };
    if (!deps.adminOnly(request)) {
      return reply.code(403).send({ error: { code: 'FORBIDDEN', message: 'admin only' } });
    }
    const parsed = parseBody(reply, RetentionSchema, request.body);
    if (!parsed.ok) return;
    if (parsed.data.organizationId !== id) {
      return reply.code(422).send({ error: { code: 'BAD_REQUEST', message: 'organizationId mismatch' } });
    }
    deps.sweeper.setRetentionDaysFor(id, parsed.data.days);
    return reply.send({ organizationId: id, retentionDays: parsed.data.days });
  });

  app.post('/api/orgs/:id/retention/sweep', async (request, reply) => {
    const { id } = request.params as { id: string };
    if (!deps.adminOnly(request)) {
      return reply.code(403).send({ error: { code: 'FORBIDDEN', message: 'admin only' } });
    }
    const results = deps.sweeper.sweepOnce(id);
    return reply.send({ swept: results });
  });
}
