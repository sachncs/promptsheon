import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import {
  CreateDatasetSchema,
  PaginationSchema,
} from '@promptsheon/shared';
import type { DatasetRepo } from '../repos/dataset.js';
import { parseBody, parseQuery } from './validate.js';

const ListQuerySchema = PaginationSchema.extend({
  capabilityId: z.string().uuid().optional(),
});

const CreateCaseSchema = z.object({
  inputs: z.string().min(1),
  expected: z.string().min(1),
  description: z.string().max(2000).optional().default(''),
});

export function registerDatasetRoutes(app: FastifyInstance, repo: DatasetRepo) {
  app.get('/api/datasets', async (request, reply) => {
    const parsed = parseQuery(reply, ListQuerySchema, request.query);
    if (!parsed.ok) return;
    const { capabilityId, page, pageSize } = parsed.data;
    if (capabilityId) return reply.send(repo.findByCapabilityId(capabilityId));
    return reply.send(repo.findMany({ page, pageSize }));
  });

  app.get('/api/datasets/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = repo.findById(id);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.post('/api/datasets', async (request, reply) => {
    const parsed = parseBody(reply, CreateDatasetSchema, request.body);
    if (!parsed.ok) return;
    const item = repo.create(parsed.data);
    return reply.code(201).send(item);
  });

  app.delete('/api/datasets/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    repo.delete(id);
    return reply.code(204).send();
  });

  app.get('/api/datasets/:id/cases', async (request, reply) => {
    const { id } = request.params as { id: string };
    return reply.send(repo.findCases(id));
  });

  app.post('/api/datasets/:id/cases', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, CreateCaseSchema, request.body);
    if (!parsed.ok) return;
    const item = repo.addCase(id, parsed.data);
    return reply.code(201).send(item);
  });

  app.delete('/api/datasets/:datasetId/cases/:caseId', async (request, reply) => {
    const { caseId } = request.params as { datasetId: string; caseId: string };
    repo.deleteCase(caseId);
    return reply.code(204).send();
  });
}
