import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { CreateScheduleSchema, PaginationSchema } from '@promptsheon/shared';
import type { ScheduleRepo } from '../repos/schedule.js';
import { parseBody, parseParams, parseQuery } from './validate.js';
import { nextCronFire } from '../scheduler/cron.js';

const UpdateScheduleSchema = z.object({
  cron: z.string().min(1).optional(),
  enabled: z.boolean().optional(),
  nextFireAt: z.string().optional(),
});
const ScheduleParamsSchema = z.object({ id: z.string().trim().min(1).max(255) });

export function registerScheduleRoutes(app: FastifyInstance, repo: ScheduleRepo) {
  app.get('/api/schedules', async (request, reply) => {
    const parsed = parseQuery(reply, PaginationSchema, request.query);
    if (!parsed.ok) return;
    return reply.send(repo.findMany(parsed.data));
  });

  app.get('/api/schedules/:id', async (request, reply) => {
    const parsedParams = parseParams(reply, ScheduleParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const item = repo.findById(id);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.post('/api/schedules', async (request, reply) => {
    const parsed = parseBody(reply, CreateScheduleSchema, request.body);
    if (!parsed.ok) return;
    try {
      nextCronFire(parsed.data.cron, new Date());
    } catch (error) {
      return reply.code(422).send({ error: { code: 'INVALID_CRON', message: error instanceof Error ? error.message : 'Invalid cron expression' } });
    }
    const item = repo.create(parsed.data);
    return reply.code(201).send(item);
  });

  app.put('/api/schedules/:id', async (request, reply) => {
    const parsedParams = parseParams(reply, ScheduleParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const parsed = parseBody(reply, UpdateScheduleSchema, request.body);
    if (!parsed.ok) return;
    if (parsed.data.cron) {
      try {
        nextCronFire(parsed.data.cron, new Date());
      } catch (error) {
        return reply.code(422).send({ error: { code: 'INVALID_CRON', message: error instanceof Error ? error.message : 'Invalid cron expression' } });
      }
    }
    const item = repo.update(id, parsed.data);
    return reply.send(item);
  });

  app.delete('/api/schedules/:id', async (request, reply) => {
    const parsedParams = parseParams(reply, ScheduleParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    repo.delete(id);
    return reply.code(204).send();
  });
}
