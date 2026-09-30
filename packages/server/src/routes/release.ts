import type { FastifyInstance, FastifyReply, FastifyRequest } from 'fastify';
import { z } from 'zod';
import {
  CreateReleaseSchema,
  PaginationSchema,
} from '@promptsheon/shared';
import type { ReleaseRepo } from '../repos/release.js';
import type { ReleaseOverlayRepo } from '../repos/release-overlay.js';
import { ManifestRepo, computeManifestHashFromJson } from '../repos/manifest.js';
import { parseBody, parseParams, parseQuery } from './validate.js';
import { AuditChain } from '../audit/chain.js';
import { createPublicKey, randomUUID, verify } from 'node:crypto';
import { selectByCanary } from '../application/canary-routing.js';
import {
  approvalGate,
  InvalidReleaseTransitionError,
  ReleaseApprovalRequiredError,
  ReleaseNotFoundError,
  ReleaseService,
} from '../application/release-service.js';
import type { CanaryRollbackService } from '../application/canary-rollback-service.js';
import type { SigningKeyRepo } from '../repos/signing-key.js';
import { releaseManifestHash, signedReleaseMessage } from '../application/release-signing.js';
import { registerFallbackRouteDoc } from '../openapi.js';

export { approvalGate } from '../application/release-service.js';

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
const SignReleaseSchema = z.object({
  keyId: z.string().trim().min(1).max(255),
  signature: z.string().min(1),
  signedAt: z.string().datetime({ offset: true }),
});

const OverlaySchema = z.object({
  patch: z.record(z.string(), z.unknown()),
});

const EnvironmentQuerySchema = z.object({
  environment: z.string().min(1).max(60).default('prod'),
});

const CanaryRuleSchema = z.object({
  percent: z.number().int().min(0).max(100),
  segmentExpr: z.string().optional(),
  windowSeconds: z.number().int().min(0).max(86400).optional(),
});

const ReleaseParamsSchema = z.object({ id: z.string().uuid() });

function actorOf(request: FastifyRequest): string {
  // Auth-disabled local development uses a synthetic principal. Transition
  // rows still require a real users.id, so use the seeded system actor for
  // persistence while preserving the principal for authorization.
  return request.userId === 'development' ? 'api' : request.userId ?? 'api';
}

function organizationOf(request: FastifyRequest): string | null {
  return request.orgContext?.orgId ?? request.agentOrgId ?? null;
}

function requireOrganization(request: FastifyRequest, reply: FastifyReply): string | null {
  const organizationId = organizationOf(request);
  if (organizationId) return organizationId;
  void reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
  return null;
}

/**
 * Select a release for an invocation using per-request random canary split.
 * Each active release in the (capability, env) pool gets weight = canaryPercent.
 * Falls back to the only active release if there's only one.
 */
export function registerReleaseRoutes(
  app: FastifyInstance,
  repo: ReleaseRepo,
  deps: { manifestRepo: ManifestRepo; auditChain: AuditChain; overlayRepo: ReleaseOverlayRepo; releaseService: ReleaseService; signingKeyRepo: SigningKeyRepo; canaryRollbackService?: CanaryRollbackService },
) {
  registerFallbackRouteDoc('post', '/api/releases/:id/sign');
  registerFallbackRouteDoc('post', '/api/releases/:id/auto-rollback');

  app.get('/api/releases', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsed = parseQuery(reply, ListQuerySchema, request.query);
    if (!parsed.ok) return;
    const { capabilityId, status, page, pageSize } = parsed.data;
    if (capabilityId) return reply.send(repo.findByCapabilityIdInOrg(capabilityId, organizationId));
    return reply.send(repo.findManyInOrg(organizationId, { page, pageSize, status }));
  });

  app.get('/api/releases/:id', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsedParams = parseParams(reply, ReleaseParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const item = repo.findByIdInOrg(id, organizationId);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.get('/api/releases/:id/transitions', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsedParams = parseParams(reply, ReleaseParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    if (!repo.findByIdInOrg(id, organizationId)) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'release not found' } });
    }
    return reply.send(repo.listTransitions(id));
  });

  app.get('/api/releases/:id/notes', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsedParams = parseParams(reply, ReleaseParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const item = repo.findByIdInOrg(id, organizationId);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'release not found' } });
    const previous = repo.findPreviousActiveInOrg(
      item.capabilityId,
      item.environment,
      item.capabilityVersion,
      organizationId,
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
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsed = parseBody(reply, CreateBodySchema, request.body);
    if (!parsed.ok) return;
    let manifestHash: string;
    try {
      manifestHash = computeManifestHashFromJson(parsed.data.manifest);
    } catch {
      return reply.code(422).send({ error: { code: 'INVALID_MANIFEST', message: 'release manifest must be a valid JSON object' } });
    }
    const item = repo.createInOrg(parsed.data, organizationId);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'capability not found' } });

    // Register the release manifest under its content-addressed identity so
    // maker-checker approvals, activation, and signing use the same hash.
    try {
      deps.manifestRepo.registerFromRaw({
        capabilityId: parsed.data.capabilityId,
        version: parsed.data.capabilityVersion,
        manifestHash,
        manifestJson: parsed.data.manifest,
        createdBy: parsed.data.createdBy,
      });
    } catch (err) {
      app.log.error({ err }, 'release manifest_dag upsert failed (non-fatal)');
    }

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
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsedParams = parseParams(reply, ReleaseParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const parsed = parseBody(reply, TransitionSchema, request.body);
    if (!parsed.ok) return;
    const existing = repo.findByIdInOrg(id, organizationId);
    if (!existing) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'release not found' } });
    try {
      const item = deps.releaseService.transition({
        releaseId: id,
        organizationId,
        actorId: actorOf(request),
        to: parsed.data.to,
        reason: parsed.data.reason,
      });
      return reply.send(item);
    } catch (error) {
      if (error instanceof ReleaseNotFoundError) {
        return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'release not found' } });
      }
      if (error instanceof InvalidReleaseTransitionError) {
        return reply.code(422).send({ error: { code: 'INVALID_TRANSITION', message: error.message } });
      }
      if (error instanceof ReleaseApprovalRequiredError) {
        return reply.code(409).send({ error: { code: 'APPROVAL_REQUIRED', message: error.message } });
      }
      throw error;
    }
  });

  app.post('/api/releases/:id/sign', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsedParams = parseParams(reply, ReleaseParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const parsed = parseBody(reply, SignReleaseSchema, request.body);
    if (!parsed.ok) return;
    const release = repo.findByIdInOrg(parsedParams.data.id, organizationId);
    if (!release) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'release not found' } });
    const key = deps.signingKeyRepo.findByIdInOrg(parsed.data.keyId, organizationId);
    if (!key || key.deactivatedAt) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'signing key not found' } });
    const message = signedReleaseMessage({ releaseId: release.id, manifestHash: releaseManifestHash(release.manifest), timestamp: parsed.data.signedAt });
    let valid = false;
    try {
      valid = verify(null, message, createPublicKey(key.publicKeyPem), Buffer.from(parsed.data.signature, 'base64'));
    } catch {
      valid = false;
    }
    if (!valid) return reply.code(422).send({ error: { code: 'BAD_SIGNATURE', message: 'signature failed verification' } });
    const updated = repo.attachSignatureInOrg({
      releaseId: release.id,
      organizationId,
      signature: parsed.data.signature,
      signedKeyId: key.id,
      signedAt: parsed.data.signedAt,
    });
    if (!updated) return reply.code(409).send({ error: { code: 'SIGNATURE_CONFLICT', message: 'release signature could not be persisted' } });
    return reply.send(updated);
  });

  app.put('/api/releases/:id/overlay', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsedParams = parseParams(reply, ReleaseParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const parsed = parseBody(reply, OverlaySchema, request.body);
    if (!parsed.ok) return;
    if (!repo.findByIdInOrg(id, organizationId)) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'release not found' } });
    }
    const parsedQuery = parseQuery(reply, EnvironmentQuerySchema, request.query);
    if (!parsedQuery.ok) return;
    const { environment: env } = parsedQuery.data;
    const overlay = deps.overlayRepo.upsert(id, env, parsed.data.patch);
    return reply.send({ id: overlay.releaseId, environment: overlay.environment, patch: overlay.patch });
  });

  app.get('/api/releases/:id/overlay', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsedParams = parseParams(reply, ReleaseParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    if (!repo.findByIdInOrg(id, organizationId)) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'release not found' } });
    }
    const parsedQuery = parseQuery(reply, EnvironmentQuerySchema, request.query);
    if (!parsedQuery.ok) return;
    const { environment: env } = parsedQuery.data;
    return reply.send({ id, environment: env, patch: deps.overlayRepo.get(id, env)?.patch ?? {} });
  });

  app.put('/api/releases/:id/canary-rule', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsedParams = parseParams(reply, ReleaseParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const parsed = parseBody(reply, CanaryRuleSchema, request.body);
    if (!parsed.ok) return;
    const current = repo.findByIdInOrg(id, organizationId);
    if (!current) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'release not found' } });
    }
    if (current.status !== 'canary' && current.status !== 'active') {
      return reply.code(409).send({ error: { code: 'INVALID_CANARY_STATE', message: 'only canary or active releases can receive traffic' } });
    }
    const updated = repo.updateCanaryPercentInOrg(id, organizationId, parsed.data.percent);
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

  app.put('/api/releases/:id/canary', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsedParams = parseParams(reply, ReleaseParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const parsed = parseBody(reply, CanaryBodySchema, request.body);
    if (!parsed.ok) return;
    const item = repo.findByIdInOrg(id, organizationId);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    if (item.status !== 'canary' && item.status !== 'active') {
      return reply.code(409).send({ error: { code: 'INVALID_CANARY_STATE', message: 'only canary or active releases can receive traffic' } });
    }
    const updated = repo.updateCanaryPercentInOrg(id, organizationId, parsed.data.percent);
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
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsedParams = parseParams(reply, ReleaseParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const current = repo.findByIdInOrg(id, organizationId);
    if (!current) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });

    const parsed = parseBody(reply, RollbackBodySchema, request.body ?? {});
    if (!parsed.ok) return;
    const target = parsed.data.toReleaseId
      ? repo.findByIdInOrg(parsed.data.toReleaseId, organizationId)
      : repo.findPreviousActiveInOrg(current.capabilityId, current.environment, current.capabilityVersion, organizationId);

    if (!target) {
      return reply.code(404).send({ error: { code: 'NO_PREVIOUS_RELEASE', message: 'No previous active release found for rollback' } });
    }
    if (target.id === current.id) {
      return reply.code(400).send({ error: { code: 'INVALID_ROLLBACK', message: 'Cannot rollback to the current release' } });
    }

    const result = repo.rollbackAtomicallyInOrg(current.id, target.id, organizationId);
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

  app.post('/api/releases/:id/auto-rollback', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsedParams = parseParams(reply, ReleaseParamsSchema, request.params);
    if (!parsedParams.ok) return;
    if (!deps.canaryRollbackService) return reply.code(503).send({ error: { code: 'UNAVAILABLE', message: 'automatic rollback is not configured' } });
    return reply.send(deps.canaryRollbackService.assess(parsedParams.data.id, organizationId, actorOf(request)));
  });
}
