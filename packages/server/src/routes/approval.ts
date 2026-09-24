import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { parseBody, parseParams, parseQuery } from './validate.js';
import type { ReleaseRepo } from '../repos/release.js';
import type { ManifestRepo } from '../repos/manifest.js';
import { NotFoundError } from '@promptsheon/shared';

const ReleaseVoteSchema = z.object({
  decision: z.enum(['approve', 'reject']),
  comment: z.string().max(1000).optional().default(''),
});

const ApprovalQuerySchema = z.object({
  releaseId: z.string().min(1).max(200),
});
const ReleaseApprovalParamsSchema = z.object({ releaseId: z.string().trim().min(1).max(255) });

interface RequestUserContext {
  userId?: string;
  agentOrgId?: string;
  orgContext?: { orgId?: string; organizationId?: string };
}

function actorOf(request: unknown): string {
  const ctx = (request as RequestUserContext | undefined) ?? {};
  return ctx.userId ?? 'system';
}

function orgOf(request: unknown): string | null {
  const ctx = (request as RequestUserContext | undefined) ?? {};
  return ctx.orgContext?.orgId ?? ctx.orgContext?.organizationId ?? ctx.agentOrgId ?? null;
}

/**
 * Register approval-related HTTP routes.
 *
 * The release approval API exposes:
 *   GET  /api/approvals?releaseId=<id>   → 200 { releaseId, approvals, ... }
 *   POST /api/releases/:releaseId/approvals   body { decision, comment? }
 *
 * The release-keyed POST forwards through to the manifest-maker-checker
 * flow (via the release's stored manifest hash) so approvals and
 * activations share the same governance gate.
 */
export function registerApprovalRoutes(
  app: FastifyInstance,
  deps: { releaseRepo: ReleaseRepo; manifestRepo: ManifestRepo },
) {
  app.get('/api/approvals/pending', async (request, reply) => {
    const organizationId = orgOf(request);
    if (!organizationId) return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    const releases = deps.releaseRepo.findManyInOrg(organizationId, { page: 1, pageSize: 100, status: 'review' }).items;
    const approvals = releases.map((release) => {
      const manifestHash = deps.releaseRepo.computeManifestHash(release.manifest);
      return {
        releaseId: release.id,
        manifestHash,
        approvals: deps.manifestRepo.findApprovals(manifestHash),
        updatedAt: release.createdAt,
      };
    });
    return reply.send({ approvals });
  });

  app.get('/api/approvals', async (request, reply) => {
    const parsed = parseQuery(reply, ApprovalQuerySchema, request.query);
    if (!parsed.ok) return;
    const { releaseId } = parsed.data;
    const organizationId = orgOf(request);
    if (!organizationId) return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    if (!deps.releaseRepo.findByIdInOrg(releaseId, organizationId)) {
      return reply.send({ releaseId, votes: '', distinctApprovers: 0, approvals: [] });
    }
    const release = deps.releaseRepo.findByIdInOrg(releaseId, organizationId);
    if (!release) return reply.send({ releaseId, distinctApprovers: 0, approvals: [] });
    const manifestHash = deps.releaseRepo.computeManifestHash(release.manifest);
    return reply.send({
      releaseId,
      manifestHash,
      distinctApprovers: deps.manifestRepo.countDistinctApprovers(manifestHash),
      approvals: deps.manifestRepo.findApprovals(manifestHash),
    });
  });

  app.post('/api/releases/:releaseId/approvals', async (request, reply) => {
    const parsedParams = parseParams(reply, ReleaseApprovalParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { releaseId } = parsedParams.data;
    const organizationId = orgOf(request);
    if (!organizationId) return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    const parsed = parseBody(reply, ReleaseVoteSchema, request.body);
    if (!parsed.ok) return;

    const release = deps.releaseRepo.findByIdInOrg(releaseId, organizationId);
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
