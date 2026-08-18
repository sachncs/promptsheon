import type { FastifyInstance } from 'fastify';
import type { DatasetRepo } from '../repos/dataset.js';

export function registerDatasetRoutes(app: FastifyInstance, repo: DatasetRepo) {
  app.get('/api/datasets', async (request, reply) => {
    const { capabilityId, page = 1, pageSize = 20 } = request.query as { capabilityId?: string; page?: number; pageSize?: number };
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
    const data = request.body as { capabilityId: string; name: string; description?: string };
    const item = repo.create(data);
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
    const data = request.body as { inputs: string; expected: string; description?: string };
    const item = repo.addCase(id, data);
    return reply.code(201).send(item);
  });

  app.delete('/api/datasets/:datasetId/cases/:caseId', async (request, reply) => {
    const { caseId } = request.params as { datasetId: string; caseId: string };
    repo.deleteCase(caseId);
    return reply.code(204).send();
  });
}
