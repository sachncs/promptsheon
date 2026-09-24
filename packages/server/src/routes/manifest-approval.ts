import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { NotFoundError } from '@promptsheon/shared';
import type { ManifestApprovalService } from '../application/manifest-approval-service.js';
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
export function registerManifestApprovalRoutes(
  app: FastifyInstance,
  deps: { service: ManifestApprovalService },
) {
  app.post('/api/manifests/:hash/approve', async (request, reply) => {
    const { hash } = request.params as { hash: string };
    const parsed = parseBody(reply, ManifestApprovalSchema, request.body);
    if (!parsed.ok) return;

    const summary = deps.service.vote(hash, parsed.data.userId, 'approve', parsed.data.comment);
    if (!summary) throw new NotFoundError('manifest', hash);
    return reply.send(summary);
  });

  app.post('/api/manifests/:hash/reject', async (request, reply) => {
    const { hash } = request.params as { hash: string };
    const parsed = parseBody(reply, ManifestRejectionSchema, request.body);
    if (!parsed.ok) return;

    const summary = deps.service.vote(hash, parsed.data.userId, 'reject', parsed.data.comment);
    if (!summary) throw new NotFoundError('manifest', hash);
    return reply.send(summary);
  });

  app.get('/api/manifests/:hash/approvals', async (request, reply) => {
    const { hash } = request.params as { hash: string };
    const summary = deps.service.get(hash);
    if (!summary) throw new NotFoundError('manifest', hash);
    return reply.send(summary);
  });
}
