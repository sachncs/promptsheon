import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type { ExecutionRepo } from '../repos/execution.js';
import {
  ExecutionReplayService,
  ReplayManifestMissingError,
  ReplayNotFoundError,
} from '../application/execution-replay-service.js';
import { ReplayInputsUnavailableError } from '../repos/execution.js';
import type { ExecutionService } from '../application/execution-service.js';
import { parseBody, parseParams, parseQuery } from './validate.js';
import { NotFoundError } from '@promptsheon/shared';
import type { SseHub } from '../sse/hub.js';
import { ExecutionSseStreamer } from '../sse/streamer.js';

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

const ExecutionParamsSchema = z.object({
  id: z.string().min(1).max(200),
});

function organizationIdOf(request: { orgContext?: { orgId?: string }; agentOrgId?: string }): string | null {
  return request.orgContext?.orgId ?? request.agentOrgId ?? null;
}

function requireOrganization(
  request: { orgContext?: { orgId?: string }; agentOrgId?: string },
  reply: { code: (status: number) => { send: (body: unknown) => unknown } },
): string | null {
  const organizationId = organizationIdOf(request);
  if (organizationId) return organizationId;
  void reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
  return null;
}

export function registerExecutionRoutes(
  app: FastifyInstance,
  deps: {
    executionRepo: ExecutionRepo;
    executionService: ExecutionService;
    replayService: ExecutionReplayService;
    sseHub?: SseHub;
  },
) {
  app.get('/api/executions', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsed = parseQuery(reply, ListExecutionsQuerySchema, request.query);
    if (!parsed.ok) return;
    const { capabilityVersionId, page, pageSize } = parsed.data;
    if (capabilityVersionId) return reply.send(deps.executionRepo.findByVersionIdInOrg(capabilityVersionId, organizationId, { page, pageSize }));
    return reply.send(deps.executionRepo.findManyInOrg(organizationId, { page, pageSize }));
  });

  app.get('/api/executions/:id', async (request, reply) => {
    const parsed = parseParams(reply, ExecutionParamsSchema, request.params);
    if (!parsed.ok) return;
    const { id } = parsed.data;
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const item = deps.executionRepo.findByIdInOrg(id, organizationId);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.post('/api/executions', async (request, reply) => {
    const parsed = parseBody(reply, ExecuteManifestSchema, request.body);
    if (!parsed.ok) return;
    const { manifestHash, inputs, environment, traceId } = parsed.data;
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;

    const executionId = crypto.randomUUID();

    // Streaming mode: when the client opts into SSE, set up a
    // streamer before the executor runs so early events aren't
    // missed. Buffered mode (default) returns the same JSON shape
    // as before.
    const accept = request.headers['accept'] ?? '';
    const wantsStream = typeof accept === 'string' && accept.includes('text/event-stream');
    const streamer =
      wantsStream && deps.sseHub ? new ExecutionSseStreamer(reply, deps.sseHub, executionId) : null;
    if (streamer) {
      streamer.open();
      // Client disconnect — close the streamer so the hub listener
      // is released and the response is ended.
      request.raw.on('close', () => streamer.close());
    }

    const controller = new AbortController();
    request.raw.on('close', () => {
      if (!controller.signal.aborted) controller.abort();
    });
    let trace;
    try {
      const result = await deps.executionService.run(manifestHash, organizationId, {
        executionId,
        inputs,
        environment,
        traceId,
        signal: controller.signal,
      });
      if (result.kind === 'manifest-not-found') throw new NotFoundError('manifest', manifestHash);
      if (result.kind === 'no-active-release') {
        if (streamer) {
          streamer.sendDone();
          streamer.close();
          return reply;
        }
        return reply.code(404).send({ error: { code: 'NO_ACTIVE_RELEASE', message: 'No active release for manifest' } });
      }
      trace = result;
    } catch (err) {
      // SSE: don't let the global error handler try to write a 500
      // — the response stream is already open.
      if (streamer) {
        streamer.sendDone();
        streamer.close();
        return reply;
      }
      throw err;
    }
    if (streamer) {
      // Final summary frame so consumers don't need to also fetch
      // /api/executions/:id. Closes the streamer afterwards.
      streamer.sendDone();
      streamer.close();
      return reply;
    }

    return reply.send({ ...trace.trace, pickedReleaseId: trace.pickedReleaseId });
  });

  /**
   * Time-travel debugging: re-run a past execution with the same
   * manifest, model, environment, and inputs. Returns the new
   * execution linked to the original via `replay_of`, plus a per-node
   * diff summary so callers can see what changed.
   *
   * Errors:
   *  - 404: original execution not found, or its manifest link is broken.
   *  - 409: original is pre-migration 049 and inputs were not captured.
   *  - 410: original's manifest is no longer in the CAS.
   *  - 422: the original is replayed against a manifest that's now invalid.
   */
  app.post('/api/executions/:id/replay', async (request, reply) => {
    const parsed = parseParams(reply, ExecutionParamsSchema, request.params);
    if (!parsed.ok) return;
    const { id } = parsed.data;
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    if (!deps.executionRepo.findByIdInOrg(id, organizationId)) {
      return reply.code(404).send({ error: { code: 'EXECUTION_NOT_FOUND', message: `execution ${id} not found` } });
    }
    try {
      const result = await deps.replayService.replay(id, organizationId);
      return reply.code(201).send({
        replayExecutionId: result.replayed.id,
        replayOf: result.original.id,
        outcome: result.outcome,
        original: result.original,
        replayed: result.replayed,
        diff: result.diff,
      });
    } catch (err) {
      if (err instanceof ReplayInputsUnavailableError) {
        return reply.code(409).send({
          error: {
            code: 'REPLAY_INPUTS_UNAVAILABLE',
            message: err.message,
          },
        });
      }
      if (err instanceof ReplayNotFoundError) {
        return reply.code(404).send({
          error: { code: 'EXECUTION_NOT_FOUND', message: err.message },
        });
      }
      if (err instanceof ReplayManifestMissingError) {
        return reply.code(410).send({
          error: {
            code: 'MANIFEST_GONE',
            message: err.message,
            manifestHash: err.manifestHash,
          },
        });
      }
      throw err;
    }
  });

  /**
   * List the replay history for an execution. Returns every replay
   * attempt (success, divergence, failure) with its outcome and diff.
   */
  app.get('/api/executions/:id/replays', async (request, reply) => {
    const parsed = parseParams(reply, ExecutionParamsSchema, request.params);
    if (!parsed.ok) return;
    const { id } = parsed.data;
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    if (!deps.executionRepo.findByIdInOrg(id, organizationId)) {
      return reply.code(404).send({
        error: { code: 'EXECUTION_NOT_FOUND', message: `execution ${id} not found` },
      });
    }
    return reply.send({ items: deps.executionRepo.findReplaysByOriginal(id) });
  });
}
