import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type { ExecutionRepo } from '../repos/execution.js';
import type { ReleaseRepo } from '../repos/release.js';
import type { ManifestRepo } from '../repos/manifest.js';
import type { VersionRepo } from '../repos/version.js';
import type { TraceRepo } from '../repos/trace.js';
import type { ManifestGraphExecutor } from '../agents/executor/index.js';
import { selectByCanary } from './release.js';
import { parseBody, parseQuery } from './validate.js';
import type { Manifest } from '@promptsheon/shared';
import { NotFoundError } from '@promptsheon/shared';
import { createHash } from 'node:crypto';

const ListExecutionsQuerySchema = z.object({
  capabilityVersionId: z.string().min(1).optional(),
  page: z.number().int().min(1).default(1),
  pageSize: z.number().int().min(1).max(100).default(20),
});

const ExecuteManifestSchema = z.object({
  manifestHash: z.string().min(1),
  inputs: z.record(z.string(), z.unknown()),
  environment: z.string().optional().default('dev'),
  traceId: z.string().optional(),
});

const InvokeSchema = z.object({
  capabilityVersionId: z.string().min(1),
  inputs: z.record(z.string(), z.unknown()),
  environment: z.string().optional().default('dev'),
  traceId: z.string().optional(),
});

function hashInputs(inputs: Record<string, unknown>): string {
  return createHash('sha256').update(JSON.stringify(inputs)).digest('hex');
}

export function registerExecutionRoutes(
  app: FastifyInstance,
  deps: {
    executionRepo: ExecutionRepo;
    releaseRepo: ReleaseRepo;
    manifestRepo: ManifestRepo;
    versionRepo: VersionRepo;
    traceRepo: TraceRepo;
    executor: ManifestGraphExecutor;
  },
) {
  app.get('/api/executions', async (request, reply) => {
    const parsed = parseQuery(reply, ListExecutionsQuerySchema, request.query);
    if (!parsed.ok) return;
    const { capabilityVersionId, page, pageSize } = parsed.data;
    if (capabilityVersionId) return reply.send(deps.executionRepo.findByVersionId(capabilityVersionId, { page, pageSize }));
    return reply.send(deps.executionRepo.findMany({ page, pageSize }));
  });

  app.get('/api/executions/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = deps.executionRepo.findById(id);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.post('/api/executions', async (request, reply) => {
    const parsed = parseBody(reply, ExecuteManifestSchema, request.body);
    if (!parsed.ok) return;
    const { manifestHash, inputs, environment, traceId } = parsed.data;
    const manifest = deps.manifestRepo.findByHash(manifestHash);
    if (!manifest) {
      throw new NotFoundError('manifest', manifestHash);
    }

    // Canary routing: when multiple active releases exist for this
    // manifest, distribute traffic by canaryPercent. With 0 or 1
    // active release, no routing is needed.
    const activeReleases = deps.releaseRepo.findActiveByManifestHash(manifestHash);
    const pickedReleaseId = selectByCanary(
      activeReleases.map((r) => ({ id: r.id, canaryPercent: r.canaryPercent })),
    );
    if (activeReleases.length === 0) {
      return reply.code(404).send({ error: { code: 'NO_ACTIVE_RELEASE', message: 'No active release for manifest' } });
    }

    const executionId = crypto.randomUUID();
    const traceRun = deps.traceRepo.startRun({
      organizationId: 'unscoped',
      executionId,
      environment,
      name: `manifest:${manifestHash.slice(0, 12)}`,
      model: manifest.model?.modelId ?? null,
      attributes: { manifestHash, route: '/api/executions' },
    });
    const controller = new AbortController();
    request.raw.on('close', () => {
      if (!controller.signal.aborted) controller.abort();
    });
    const trace = await deps.executor.execute(manifestHash, manifest, {
      executionId,
      inputs,
      environment,
      traceId,
      signal: controller.signal,
      traceRunId: traceRun.id,
    });
    deps.traceRepo.finalize(traceRun.id, trace.status === 'completed' ? 'success' : 'error', {
      tokens: trace.totalTokens,
      costUsd: trace.totalCost,
    });
    deps.executionRepo.create({
      capabilityVersionId: manifest.id,
      inputs: hashInputs(inputs),
      outputs: JSON.stringify(trace.nodeResults),
      model: manifest.model.modelId,
      provider: manifest.model.provider,
      latencyMs: trace.totalLatencyMs,
      costUsd: trace.totalCost,
      promptTokens: 0,
      completionTokens: 0,
      totalTokens: trace.totalTokens,
      error: trace.error ?? '',
      traceId: traceId ?? executionId,
      environment,
    });
    return reply.send({ ...trace, pickedReleaseId });
  });

  // Legacy alias kept for SDK + curl examples. Accepts a
  // capabilityVersionId (version row id), looks up its manifest
  // hash, and forwards to the canonical execute path so SDK
  // consumers can still use the historic /api/invoke contract.
  app.post('/api/invoke', async (request, reply) => {
    const parsed = parseBody(reply, InvokeSchema, request.body);
    if (!parsed.ok) return;
    const { capabilityVersionId, inputs, environment, traceId } = parsed.data;
    const version = deps.versionRepo.findById(capabilityVersionId);
    if (!version) {
      return reply.code(404).send({
        error: { code: 'VERSION_NOT_FOUND', message: 'capabilityVersionId not found' },
      });
    }
    const manifestHash = version.manifestHash;
    if (!manifestHash) {
      return reply.code(409).send({
        error: { code: 'NO_MANIFEST_HASH', message: 'version has no manifestHash' },
      });
    }
    // Synthesise a forward-compatible execute request and call
    // the canonical POST /api/executions handler by inlining its
    // body. We avoid a self-fetch (which would re-introduce auth
    // + a round-trip) by reusing the same controller below.
    const manifest = deps.manifestRepo.findByHash(manifestHash);
    if (!manifest) {
      throw new NotFoundError('manifest', manifestHash);
    }
    const activeReleases = deps.releaseRepo.findActiveByManifestHash(manifestHash);
    if (activeReleases.length === 0) {
      return reply
        .code(404)
        .send({ error: { code: 'NO_ACTIVE_RELEASE', message: 'No active release for manifest' } });
    }
    const pickedReleaseId = selectByCanary(
      activeReleases.map((r) => ({ id: r.id, canaryPercent: r.canaryPercent })),
    );
    const executionId = crypto.randomUUID();
    const traceRun = deps.traceRepo.startRun({
      organizationId: 'unscoped',
      executionId,
      environment,
      name: `manifest:${manifestHash.slice(0, 12)}`,
      model: manifest.model?.modelId ?? null,
      attributes: { manifestHash, route: '/api/invoke' },
    });
    const controller = new AbortController();
    request.raw.on('close', () => {
      if (!controller.signal.aborted) controller.abort();
    });
    const trace = await deps.executor.execute(manifestHash, manifest, {
      executionId,
      inputs,
      environment,
      traceId,
      signal: controller.signal,
      traceRunId: traceRun.id,
    });
    deps.traceRepo.finalize(traceRun.id, trace.status === 'completed' ? 'success' : 'error', {
      tokens: trace.totalTokens,
      costUsd: trace.totalCost,
    });
    deps.executionRepo.create({
      capabilityVersionId: manifest.id,
      inputs: hashInputs(inputs),
      outputs: JSON.stringify(trace.nodeResults),
      model: manifest.model.modelId,
      provider: manifest.model.provider,
      latencyMs: trace.totalLatencyMs,
      costUsd: trace.totalCost,
      promptTokens: 0,
      completionTokens: 0,
      totalTokens: trace.totalTokens,
      error: trace.error ?? '',
      traceId: traceId ?? executionId,
      environment,
    });
    return reply.send({ ...trace, pickedReleaseId, capabilityVersionId });
  });
}