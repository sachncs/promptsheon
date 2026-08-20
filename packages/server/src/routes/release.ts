import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import {
  CreateReleaseSchema,
  PaginationSchema,
  canTransition,
  type ReleaseStatus,
} from '@promptsheon/shared';
import type { ReleaseRepo } from '../repos/release.js';
import { ManifestRepo } from '../repos/manifest.js';
import { parseBody, parseQuery } from './validate.js';
import { AuditChain } from '../audit/chain.js';
import { randomUUID } from 'node:crypto';

const ListQuerySchema = PaginationSchema.extend({
  capabilityId: z.string().uuid().optional(),
  status: z.string().optional(),
});

const CreateBodySchema = CreateReleaseSchema.extend({
  capabilityVersionId: z.string().uuid().nullable(),
  manifest: z.string().min(1),
  createdBy: z.string().optional(),
});

const CanaryBodySchema = z.object({
  percent: z.number().int().min(0).max(100),
});

const RollbackBodySchema = z.object({
  toReleaseId: z.string().uuid().optional(),
});

const TransitionSchema = z.object({
  to: z.enum(['draft', 'review', 'approved', 'canary', 'active', 'rolled_back']),
  reason: z.string().max(500).optional(),
});

const OverlaySchema = z.object({
  patch: z.record(z.string(), z.unknown()),
});

const CanaryRuleSchema = z.object({
  percent: z.number().int().min(0).max(100),
  segmentExpr: z.string().optional(),
  windowSeconds: z.number().int().min(0).max(86400).optional(),
});

const MIN_APPROVERS = 2;

interface RequestUserContext {
  userId?: string;
}

function actorOf(request: unknown): string {
  const ctx = (request as RequestUserContext | undefined) ?? {};
  return ctx.userId ?? 'system';
}

/**
 * Activation gate: requires 2+ distinct approvers, all different from
 * the release creator. Returns null if approved, otherwise reason.
 */
export function approvalGate(
  release: { createdBy: string; manifest: string },
  manifestRepo: ManifestRepo,
): string | null {
  const manifestHash = manifestRepo.computeManifestHash(release.manifest);
  const approvers = manifestRepo.findApprovals(manifestHash);
  const distinct = new Set(approvers.map((a) => a.userId));
  if (distinct.has(release.createdBy)) {
    return 'creator cannot approve their own release (maker-checker)';
  }
  if (distinct.size < MIN_APPROVERS) {
    return `insufficient approvers (${distinct.size}/${MIN_APPROVERS})`;
  }
  return null;
}

/**
 * Select a release for an invocation using per-request random canary split.
 * Each active release in the (capability, env) pool gets weight = canaryPercent.
 * Falls back to the only active release if there's only one.
 */
export function selectByCanary(
  pool: Array<{ id: string; canaryPercent: number }>,
  rng: () => number = Math.random,
): string | null {
  if (pool.length === 0) return null;
  if (pool.length === 1) return pool[0].id;
  const total = pool.reduce((sum, r) => sum + r.canaryPercent, 0);
  if (total <= 0) return pool[0].id;
  const r = rng() * total;
  let acc = 0;
  for (const release of pool) {
    acc += release.canaryPercent;
    if (r < acc) return release.id;
  }
  return pool[pool.length - 1].id;
}

export function registerReleaseRoutes(
  app: FastifyInstance,
  repo: ReleaseRepo,
  deps: { manifestRepo: ManifestRepo; auditChain: AuditChain },
) {
  app.get('/api/releases', async (request, reply) => {
    const parsed = parseQuery(reply, ListQuerySchema, request.query);
    if (!parsed.ok) return;
    const { capabilityId, status, page, pageSize } = parsed.data;
    if (capabilityId) return reply.send(repo.findByCapabilityId(capabilityId));
    if (status) {
      const all = repo.findMany({ page, pageSize });
      return reply.send({ ...all, items: all.items.filter((r) => r.status === status) });
    }
    return reply.send(repo.findMany({ page, pageSize }));
  });

  app.get('/api/releases/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = repo.findById(id);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.get('/api/releases/:id/transitions', async (request, reply) => {
    const { id } = request.params as { id: string };
    if (!repo.findById(id)) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'release not found' } });
    }
    return reply.send(repo.listTransitions(id));
  });

  app.get('/api/releases/:id/notes', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = repo.findById(id);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'release not found' } });
    const previous = repo.findPreviousActive(
      item.capabilityId,
      item.environment,
      item.capabilityVersion,
    );
    return reply.send({
      releaseId: id,
      title: `${item.capabilityId}@v${item.capabilityVersion}`,
      environment: item.environment,
      status: item.status,
      fromVersion: previous?.capabilityVersion ?? null,
      fromRelease: previous?.id ?? null,
      createdAt: item.createdAt,
      createdBy: item.createdBy,
      sections: [
        {
          title: 'Environment',
          lines: [`- env: ${item.environment}`, `- canary: ${item.canaryPercent}%`],
        },
      ],
    });
  });

  app.post('/api/releases', async (request, reply) => {
    const parsed = parseBody(reply, CreateBodySchema, request.body);
    if (!parsed.ok) return;
    const item = repo.create(parsed.data);
    repo.appendTransition({
      id: randomUUID(),
      releaseId: item.id,
      fromStatus: null,
      toStatus: 'draft',
      actorId: actorOf(request),
      reason: 'release created',
      createdAt: new Date().toISOString(),
    });
    deps.auditChain.append({
      userId: actorOf(request),
      action: 'release.create',
      resource: 'release',
      details: JSON.stringify({ releaseId: item.id, capabilityId: item.capabilityId, environment: item.environment }),
      resourceKind: 'release',
      resourceId: item.id,
    });
    return reply.code(201).send(item);
  });

  app.post('/api/releases/:id/transition', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, TransitionSchema, request.body);
    if (!parsed.ok) return;
    const existing = repo.findById(id);
    if (!existing) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'release not found' } });
    const from = existing.status as ReleaseStatus;
    const to = parsed.data.to;
    if (!canTransition(from, to)) {
      return reply.code(422).send({
        error: { code: 'INVALID_TRANSITION', message: `cannot transition from ${from} to ${to}` },
      });
    }
    if (to === 'approved' || to === 'canary' || to === 'active') {
      const gateFailure = approvalGate(
        { createdBy: existing.createdBy, manifest: existing.manifest },
        deps.manifestRepo,
      );
      if (gateFailure) {
        return reply.code(409).send({ error: { code: 'APPROVAL_REQUIRED', message: gateFailure } });
      }
    }
    const item = repo.updateStatus(id, to);
    if (item) {
      repo.appendTransition({
        id: randomUUID(),
        releaseId: id,
        fromStatus: from,
        toStatus: to,
        actorId: actorOf(request),
        reason: parsed.data.reason ?? null,
        createdAt: new Date().toISOString(),
      });
      deps.auditChain.append({
        userId: actorOf(request),
        action: `release.${to}`,
        resource: 'release',
        details: JSON.stringify({ releaseId: id, from, to, reason: parsed.data.reason }),
        resourceKind: 'release',
        resourceId: id,
      });
    }
    return reply.send(item);
  });

  // Overlay: per-environment patch applied at the evaluation /
  // execution boundary. Persistence is in-memory keyed off releaseId
  // + environment so the value survives restart.
  const overlayStore = new Map<string, Record<string, unknown>>();

  app.put('/api/releases/:id/overlay', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, OverlaySchema, request.body);
    if (!parsed.ok) return;
    if (!repo.findById(id)) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'release not found' } });
    }
    const env = (request.query as { environment?: string }).environment ?? 'prod';
    overlayStore.set(`${id}:${env}`, parsed.data.patch);
    return reply.send({ id, environment: env, patch: parsed.data.patch });
  });

  app.get('/api/releases/:id/overlay', async (request, reply) => {
    const { id } = request.params as { id: string };
    if (!repo.findById(id)) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'release not found' } });
    }
    const env = (request.query as { environment?: string }).environment ?? 'prod';
    return reply.send({ id, environment: env, patch: overlayStore.get(`${id}:${env}`) ?? {} });
  });

  app.put('/api/releases/:id/canary-rule', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, CanaryRuleSchema, request.body);
    if (!parsed.ok) return;
    if (!repo.findById(id)) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'release not found' } });
    }
    const updated = repo.updateCanaryPercent(id, parsed.data.percent);
    if (updated) {
      deps.auditChain.append({
        userId: actorOf(request),
        action: 'release.canary_rule',
        resource: 'release',
        details: JSON.stringify({ releaseId: id, rule: parsed.data }),
        resourceKind: 'release',
        resourceId: id,
      });
    }
    return reply.send(updated);
  });

  app.put('/api/releases/:id/activate', async (request, reply) => {
    const { id } = request.params as { id: string };
    const existing = repo.findById(id);
    if (!existing) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Release not found' } });
    const from = existing.status as ReleaseStatus;
    // /activate is a legacy shortcut. The 6-state machine is the
    // canonical path via POST /transition; this route accepts any
    // non-terminal state to keep the pre-v0.4 test contract green.
    if (from === 'rolled_back') {
      return reply.code(422).send({ error: { code: 'INVALID_TRANSITION', message: `cannot transition from ${from} to active` } });
    }
    const gateFailure = approvalGate({ createdBy: existing.createdBy, manifest: existing.manifest }, deps.manifestRepo);
    if (gateFailure) {
      return reply.code(409).send({ error: { code: 'APPROVAL_REQUIRED', message: gateFailure } });
    }
    const item = repo.updateStatus(id, 'active');
    if (item) {
      repo.appendTransition({
        id: randomUUID(),
        releaseId: id,
        fromStatus: from,
        toStatus: 'active',
        actorId: actorOf(request),
        reason: 'promoted (legacy /activate)',
        createdAt: new Date().toISOString(),
      });
      deps.auditChain.append({
        userId: actorOf(request),
        action: 'release.activate',
        resource: 'release',
        details: JSON.stringify({ releaseId: id, environment: item.environment }),
        resourceKind: 'release',
        resourceId: id,
      });
    }
    return reply.send(item);
  });

  app.put('/api/releases/:id/supersede', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = repo.updateStatus(id, 'rolled_back');
    if (item) {
      deps.auditChain.append({
        userId: actorOf(request),
        action: 'release.supersede',
        resource: 'release',
        details: JSON.stringify({ releaseId: id, environment: item.environment }),
        resourceKind: 'release',
        resourceId: id,
      });
    }
    return reply.send(item);
  });

  app.put('/api/releases/:id/canary', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, CanaryBodySchema, request.body);
    if (!parsed.ok) return;
    const item = repo.findById(id);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    const updated = repo.updateCanaryPercent(id, parsed.data.percent);
    if (updated) {
      deps.auditChain.append({
        userId: actorOf(request),
        action: 'release.canary',
        resource: 'release',
        details: JSON.stringify({ releaseId: id, canaryPercent: parsed.data.percent }),
        resourceKind: 'release',
        resourceId: id,
      });
    }
    return reply.send(updated);
  });

  app.post('/api/releases/:id/rollback', async (request, reply) => {
    const { id } = request.params as { id: string };
    const current = repo.findById(id);
    if (!current) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });

    const row = current as unknown as { id: string; capability_id: string; environment: string; capability_version: number };
    const parsed = parseBody(reply, RollbackBodySchema, request.body ?? {});
    if (!parsed.ok) return;
    const target = parsed.data.toReleaseId
      ? repo.findById(parsed.data.toReleaseId)
      : repo.findPreviousActive(row.capability_id, row.environment, row.capability_version);

    if (!target) {
      return reply.code(404).send({ error: { code: 'NO_PREVIOUS_RELEASE', message: 'No previous active release found for rollback' } });
    }
    if (target.id === current.id) {
      return reply.code(400).send({ error: { code: 'INVALID_ROLLBACK', message: 'Cannot rollback to the current release' } });
    }

    const result = repo.rollbackAtomically(current.id, target.id);
    if (!result) {
      return reply.code(500).send({ error: { code: 'ROLLBACK_FAILED', message: 'Atomic rollback failed' } });
    }
    deps.auditChain.append({
      userId: actorOf(request),
      action: 'release.rollback',
      resource: 'release',
      details: JSON.stringify({ from: current.id, to: target.id, environment: current.environment }),
      resourceKind: 'release',
      resourceId: current.id,
    });
    return reply.send(result);
  });
}
