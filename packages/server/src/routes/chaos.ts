import type { FastifyInstance, FastifyRequest, FastifyReply } from 'fastify';
import { z } from 'zod';
import { ChaosConfig, type FailureKind } from '../hardening/chaos.js';

const InjectSchema = z.object({
  nodeId: z.string().min(1),
  kind: z.union([z.literal('timeout'), z.literal('crash'), z.literal('rate-limit')]),
  message: z.string().optional(),
  delayMs: z.number().int().positive().optional(),
  hitCount: z.number().int().positive().optional(),
});

const ClearSchema = z.object({ nodeId: z.string().min(1) });

export interface ChaosRouteDeps {
  chaos: ChaosConfig;
  /** Returns true if the caller is authorised to inject/clear. */
  isAdmin: (request: FastifyRequest) => boolean;
}

/**
 * Register chaos admin routes. Endpoints:
 *   POST /api/admin/chaos/inject — inject a failure
 *   POST /api/admin/chaos/clear — clear one or all
 *   GET  /api/admin/chaos/list  — list active injections
 *
 * Both mutation endpoints require the caller to satisfy `isAdmin`.
 * Returns 401 (not 403) on auth failure to avoid leaking the endpoint
 * shape to non-admins.
 */
export function registerChaosRoutes(app: FastifyInstance, deps: ChaosRouteDeps): void {
  app.post('/api/admin/chaos/inject', async (request: FastifyRequest, reply: FastifyReply) => {
    if (!deps.isAdmin(request)) {
      return reply.code(401).send({ error: { code: 'UNAUTHORIZED', message: 'admin required' } });
    }
    const parsed = InjectSchema.safeParse(request.body);
    if (!parsed.success) {
      return reply.code(422).send({ error: { code: 'VALIDATION_ERROR', message: parsed.error.message } });
    }
    const { nodeId, kind, message, delayMs, hitCount } = parsed.data;
    deps.chaos.inject(nodeId, { kind: kind as FailureKind, message, delayMs, hitCount });
    return reply.code(200).send({ nodeId, kind, message, delayMs, hitCount });
  });

  app.post('/api/admin/chaos/clear', async (request: FastifyRequest, reply: FastifyReply) => {
    if (!deps.isAdmin(request)) {
      return reply.code(401).send({ error: { code: 'UNAUTHORIZED', message: 'admin required' } });
    }
    const parsed = ClearSchema.safeParse(request.body);
    if (!parsed.success) {
      return reply.code(422).send({ error: { code: 'VALIDATION_ERROR', message: parsed.error.message } });
    }
    const removed = deps.chaos.clear(parsed.data.nodeId);
    return reply.code(200).send({ nodeId: parsed.data.nodeId, removed });
  });

  app.get('/api/admin/chaos/list', async (request: FastifyRequest, reply: FastifyReply) => {
    if (!deps.isAdmin(request)) {
      return reply.code(401).send({ error: { code: 'UNAUTHORIZED', message: 'admin required' } });
    }
    return reply.code(200).send({ injections: deps.chaos.list() });
  });
}