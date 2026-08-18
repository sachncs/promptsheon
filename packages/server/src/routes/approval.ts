import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type { ApprovalRepo } from '../repos/approval.js';
import { parseBody } from './validate.js';

const UpsertApprovalSchema = z.object({
  releaseId: z.string().min(1),
  votes: z.string(),
});

export function registerApprovalRoutes(app: FastifyInstance, repo: ApprovalRepo) {
  app.get('/api/approvals/:releaseId', async (request, reply) => {
    const { releaseId } = request.params as { releaseId: string };
    const item = repo.getByReleaseId(releaseId);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.post('/api/approvals', async (request, reply) => {
    const parsed = parseBody(reply, UpsertApprovalSchema, request.body);
    if (!parsed.ok) return;
    const { releaseId, votes } = parsed.data;
    repo.upsert(releaseId, votes);
    return reply.code(201).send({ releaseId, votes });
  });
}