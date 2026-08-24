import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type { ApprovalRepo } from '../repos/approval.js';
import { parseBody } from './validate.js';
import type { ReleaseRepo } from '../repos/release.js';
import type { ManifestRepo } from '../repos/manifest.js';
import { NotFoundError } from '@promptsheon/shared';

const UpsertApprovalSchema = z.object({
  releaseId: z.string().min(1),
  votes: z.string(),
});

const ReleaseVoteSchema = z.object({
  decision: z.enum(['approve', 'reject']),
  comment: z.string().max(1000).optional().default(''),
});

interface RequestUserContext {
  userId?: string;
}

function actorOf(request: unknown): string {
  const ctx = (request as RequestUserContext | undefined) ?? {};
  return ctx.userId ?? 'system';
}

/**
 * Register approval-related HTTP routes.
 *
 * Two legacy routes are kept (used by older internal callers and tests):
 *   GET  /api/approvals/:releaseId
 *   POST /api/approvals   body { releaseId, votes }
 *
 * Two new routes the frontend uses:
 *   GET  /api/approvals?releaseId=<id>   → 200 { releaseId, votes, ... }
 *   POST /api/releases/:releaseId/approvals   body { decision, comment? }
 *
 * The release-keyed POST forwards through to the manifest-maker-checker
 * flow (via the release's stored manifest hash) so approvals and
 * activations share the same governance gate.
 */
export function registerApprovalRoutes(
  app: FastifyInstance,
  repo: ApprovalRepo,
  deps: { releaseRepo: ReleaseRepo; manifestRepo: ManifestRepo },
) {
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

  app.get('/api/approvals', async (request, reply) => {
    const releaseId = (request.query as { releaseId?: string }).releaseId;
    if (!releaseId) {
      return reply.code(400).send({ error: { code: 'MISSING_RELEASE_ID', message: 'releaseId required' } });
    }
    const item = repo.getByReleaseId(releaseId);
    if (!item) return reply.send({ releaseId, votes: '', distinctApprovers: 0, approvals: [] });
    return reply.send(item);
  });

  app.post('/api/releases/:releaseId/approvals', async (request, reply) => {
    const { releaseId } = request.params as { releaseId: string };
    const parsed = parseBody(reply, ReleaseVoteSchema, request.body);
    if (!parsed.ok) return;

    const release = deps.releaseRepo.findById(releaseId);
    if (!release) throw new NotFoundError('release', releaseId);

    const manifestHash = deps.releaseRepo.computeManifestHash(release.manifest);
    const manifest = deps.manifestRepo.findByHash(manifestHash);
    if (!manifest) {
      return reply.code(409).send({
        error: {
          code: 'MANIFEST_NOT_REGISTERED',
          message: 'release manifest is not in manifest_dag; re-create the release',
        },
      });
    }

    const voterId = actorOf(request);
    deps.manifestRepo.upsertApproval(manifestHash, voterId, parsed.data.decision, parsed.data.comment);
    const approvals = deps.manifestRepo.findApprovals(manifestHash);
    return reply.code(201).send({
      releaseId,
      decision: parsed.data.decision,
      comment: parsed.data.comment,
      distinctApprovers: deps.manifestRepo.countDistinctApprovers(manifestHash),
      approvals,
    });
  });
}