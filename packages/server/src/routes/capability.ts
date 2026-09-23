import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import {
  CreateCapabilitySchema,
  UpdateCapabilitySchema,
  PaginationSchema,
} from '@promptsheon/shared';
import type { CapabilityService } from '../application/capability-service.js';
import { parseBody, parseParams, parseQuery, sendNotFound } from './validate.js';

const IdParamsSchema = z.object({ id: z.string().uuid() });

const ListQuerySchema = PaginationSchema.extend({
  projectId: z.string().uuid().optional(),
});

export function registerCapabilityRoutes(app: FastifyInstance, service: CapabilityService) {
  app.get('/api/capabilities', async (request, reply) => {
    const parsed = parseQuery(reply, ListQuerySchema, request.query);
    if (!parsed.ok) return;
    const { projectId, page, pageSize } = parsed.data;
    if (projectId) return reply.send(service.listByProject(projectId));
    return reply.send(service.list({ page, pageSize }));
  });

  app.get('/api/capabilities/:id', async (request, reply) => {
    const parsedParams = parseParams(reply, IdParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const item = service.get(parsedParams.data.id);
    if (!item) return sendNotFound(reply, 'Capability', parsedParams.data.id);
    return reply.send(item);
  });

  app.post('/api/capabilities', async (request, reply) => {
    const parsed = parseBody(reply, CreateCapabilitySchema, request.body);
    if (!parsed.ok) return;
    const item = service.create(parsed.data);
    return reply.code(201).send(item);
  });

  app.put('/api/capabilities/:id', async (request, reply) => {
    const parsedParams = parseParams(reply, IdParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const parsed = parseBody(reply, UpdateCapabilitySchema, request.body);
    if (!parsed.ok) return;
    const item = service.update(parsedParams.data.id, parsed.data);
    if (!item) return sendNotFound(reply, 'Capability', parsedParams.data.id);
    return reply.send(item);
  });

  app.delete('/api/capabilities/:id', async (request, reply) => {
    const parsed = parseParams(reply, IdParamsSchema, request.params);
    if (!parsed.ok) return;
    if (!service.remove(parsed.data.id)) return sendNotFound(reply, 'Capability', parsed.data.id);
    return reply.code(204).send();
  });
}
