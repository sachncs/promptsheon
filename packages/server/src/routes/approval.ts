import type { FastifyInstance } from 'fastify';
import type { ApprovalRepo } from '../repos/approval.js';

export function registerApprovalRoutes(app: FastifyInstance, repo: ApprovalRepo) {
  app.get('/api/approvals/:releaseId', async (request, reply) => {
    const { releaseId } = request.params as { releaseId: string };
    const item = repo.getByReleaseId(releaseId);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.post('/api/approvals', async (request, reply) => {
    const { releaseId, votes } = request.body as { releaseId: string; votes: string };
    repo.upsert(releaseId, votes);
    return reply.code(201).send({ releaseId, votes });
  });
}
