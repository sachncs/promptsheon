import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { NotFoundError } from '@promptsheon/shared';
import type { ManifestRepo } from '../repos/manifest.js';
import { parseBody } from './validate.js';

const ManifestApprovalSchema = z.object({
  userId: z.string().min(1).max(255),
  comment: z.string().max(1000).optional().default(''),
});

const ManifestRejectionSchema = z.object({
  userId: z.string().min(1).max(255),
  comment: z.string().max(1000).optional().default(''),
});

/**
 * Maker-checker approval workflow for Manifest DAGs.
 *
 * Approval rules:
 * - Creator cannot approve own manifest (enforced at activation time)
 * - 2+ distinct approvers required to activate
 * - Same user re-voting overwrites prior vote
 *
 * The DB table manifest_approvals uses manifest_dag.id (not manifest.id)
 * as FK, so the hash → manifest_dag lookup is required first.
 */
export function registerManifestApprovalRoutes(app: FastifyInstance, deps: { manifestRepo: ManifestRepo }) {
  app.post('/api/manifests/:hash/approve', async (request, reply) => {
    const { hash } = request.params as { hash: string };
    const parsed = parseBody(reply, ManifestApprovalSchema, request.body);
    if (!parsed.ok) return;

    const manifest = deps.manifestRepo.findByHash(hash);
    if (!manifest) throw new NotFoundError('manifest', hash);

    deps.manifestRepo.upsertApproval(hash, parsed.data.userId, 'approve', parsed.data.comment);
    const approvals = deps.manifestRepo.findApprovals(hash);
    return reply.send({
      hash,
      approvals,
      distinctApprovers: deps.manifestRepo.countDistinctApprovers(hash),
    });
  });

  app.post('/api/manifests/:hash/reject', async (request, reply) => {
    const { hash } = request.params as { hash: string };
    const parsed = parseBody(reply, ManifestRejectionSchema, request.body);
    if (!parsed.ok) return;

    const manifest = deps.manifestRepo.findByHash(hash);
    if (!manifest) throw new NotFoundError('manifest', hash);

    deps.manifestRepo.upsertApproval(hash, parsed.data.userId, 'reject', parsed.data.comment);
    const approvals = deps.manifestRepo.findApprovals(hash);
    return reply.send({
      hash,
      approvals,
      distinctApprovers: deps.manifestRepo.countDistinctApprovers(hash),
    });
  });

  app.get('/api/manifests/:hash/approvals', async (request, reply) => {
    const { hash } = request.params as { hash: string };
    const manifest = deps.manifestRepo.findByHash(hash);
    if (!manifest) throw new NotFoundError('manifest', hash);
    return reply.send({
      hash,
      approvals: deps.manifestRepo.findApprovals(hash),
      distinctApprovers: deps.manifestRepo.countDistinctApprovers(hash),
    });
  });
}