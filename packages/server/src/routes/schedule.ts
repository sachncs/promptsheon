import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { CreateScheduleSchema, PaginationSchema } from '@promptsheon/shared';
import { InvalidScheduleCronError, ScheduleService } from '../application/schedule-service.js';
import { parseBody, parseParams, parseQuery } from './validate.js';

const UpdateScheduleSchema = z.object({
  cron: z.string().min(1).optional(),
  enabled: z.boolean().optional(),
  nextFireAt: z.string().optional(),
});
const ScheduleParamsSchema = z.object({ id: z.string().trim().min(1).max(255) });

export function registerScheduleRoutes(app: FastifyInstance, service: ScheduleService) {
  app.get('/api/schedules', async (request, reply) => {
    const parsed = parseQuery(reply, PaginationSchema, request.query);
    if (!parsed.ok) return;
    return reply.send(service.list(parsed.data));
  });

  app.get('/api/schedules/:id', async (request, reply) => {
    const parsedParams = parseParams(reply, ScheduleParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const item = service.get(id);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.post('/api/schedules', async (request, reply) => {
    const parsed = parseBody(reply, CreateScheduleSchema, request.body);
    if (!parsed.ok) return;
    try {
      return reply.code(201).send(service.create(parsed.data));
    } catch (error) {
      if (error instanceof InvalidScheduleCronError) {
        return reply.code(422).send({ error: { code: 'INVALID_CRON', message: error.message } });
      }
      throw error;
    }
  });

  app.put('/api/schedules/:id', async (request, reply) => {
    const parsedParams = parseParams(reply, ScheduleParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const parsed = parseBody(reply, UpdateScheduleSchema, request.body);
    if (!parsed.ok) return;
    try {
      return reply.send(service.update(id, parsed.data));
    } catch (error) {
      if (error instanceof InvalidScheduleCronError) {
        return reply.code(422).send({ error: { code: 'INVALID_CRON', message: error.message } });
      }
      throw error;
    }
  });

  app.delete('/api/schedules/:id', async (request, reply) => {
    const parsedParams = parseParams(reply, ScheduleParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    service.delete(id);
    return reply.code(204).send();
  });
}
