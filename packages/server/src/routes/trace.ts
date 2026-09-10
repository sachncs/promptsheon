import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type { TraceRepo } from '../repos/trace.js';
import { parseQuery } from './validate.js';

const ListQuerySchema = z.object({
  page: z.coerce.number().int().min(1).default(1),
  pageSize: z.coerce.number().int().min(1).max(200).default(25),
  environment: z.string().min(1).max(60).optional(),
  status: z.enum(['running', 'success', 'error']).optional(),
  nameLike: z.string().min(1).max(120).optional(),
  actorId: z.string().min(1).max(120).optional(),
  fromTime: z.string().min(1).max(80).optional(),
  toTime: z.string().min(1).max(80).optional(),
});

const RollupQuerySchema = z.object({
  days: z.coerce.number().int().min(1).max(365).default(30),
  environment: z.string().min(1).max(60).optional(),
});

interface RequestUserContext {
  userId?: string;
  orgContext?: { organizationId?: string };
}

function orgOf(request: unknown): string | null {
  const ctx = (request as RequestUserContext | undefined) ?? {};
  return ctx.orgContext?.organizationId ?? null;
}

/**
 * GET /api/traces — list trace runs for the active org, newest first.
 * GET /api/traces/:id — fetch one run with its full span tree.
 * GET /api/traces/rollup — per-day tokens + cost for the active org.
 *
 * Read-only surface; the writer paths are the executor + the
 * gateway / eval-on-trace hooks. Admin role is required so a
 * reader-tier token can't fan out to neighbour orgs' history.
 */
export function registerTraceRoutes(
  app: FastifyInstance,
  deps: { traceRepo: TraceRepo; requireAdmin: () => (request: unknown, reply: unknown) => Promise<void> },
) {
  app.get(
    '/api/traces/rollup',
    { preHandler: deps.requireAdmin() },
    async (request, reply) => {
      const orgId = orgOf(request);
      if (!orgId) {
        return reply
          .code(401)
          .send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
      }
      const parsed = parseQuery(reply, RollupQuerySchema, request.query);
      if (!parsed.ok) return;
      const { days, environment } = parsed.data;
      const items = deps.traceRepo.rollupByOrg(orgId, { days, environment });
      return reply.send({ orgId, days, environment: environment ?? null, items });
    },
  );

  app.get(
    '/api/traces',
    { preHandler: deps.requireAdmin() },
    async (request, reply) => {
      const orgId = orgOf(request);
      if (!orgId) {
        return reply
          .code(401)
          .send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
      }
      const parsed = parseQuery(reply, ListQuerySchema, request.query);
      if (!parsed.ok) return;
      const data = deps.traceRepo.listByOrg(orgId, parsed.data);
      return reply.send(data);
    },
  );

  app.get(
    '/api/traces/:id',
    { preHandler: deps.requireAdmin() },
    async (request, reply) => {
      const { id } = request.params as { id: string };
      const run = deps.traceRepo.findById(id);
      if (!run) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'trace run not found' } });
      const spans = deps.traceRepo.findSpansByRun(id);
      return reply.send({ run, spans });
    },
  );
}
