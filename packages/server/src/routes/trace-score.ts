import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type { TraceService } from '../application/trace-service.js';
import { parseBody, parseQuery } from './validate.js';

const RunAutoEvalSchema = z.object({
  judgeModel: z.string().min(1).max(120).optional(),
  judgePrompt: z.string().min(1).max(4000).optional(),
});

const ListScoresQuerySchema = z.object({
  page: z.coerce.number().int().min(1).default(1),
  pageSize: z.coerce.number().int().min(1).max(200).default(50),
});

const SummaryQuerySchema = z.object({
  days: z.coerce.number().int().min(1).max(90).default(7),
  evaluator: z.string().min(1).max(120).optional(),
});

interface RequestUserContext {
  userId?: string;
  agentOrgId?: string;
  orgContext?: { organizationId?: string; orgId?: string };
}

function orgOf(request: unknown): string | null {
  const ctx = (request as RequestUserContext | undefined) ?? {};
  return ctx.orgContext?.orgId ?? ctx.orgContext?.organizationId ?? ctx.agentOrgId ?? null;
}

/**
 * GET  /api/traces/:id/scores — list eval results for a trace_run.
 * POST /api/traces/:id/score — record one eval result manually.
 * POST /api/traces/:id/auto-eval — run every registered evaluator
 *   against the trace (used after every /api/executions).
 * GET  /api/scores/summary — org-wide eval summary for the
 *   analytics page.
 *
 * Admin role gates the read paths; manual score POSTs are open
 * because evaluators run as the system actor.
 */
export function registerTraceScoreRoutes(
  app: FastifyInstance,
  deps: { service: TraceService },
) {
  app.get('/api/traces/:id/scores', async (request, reply) => {
    const { id } = request.params as { id: string };
    const orgId = orgOf(request);
    if (!orgId) {
      return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    }
    const parsed = parseQuery(reply, ListScoresQuerySchema, request.query);
    if (!parsed.ok) return;
    const scores = deps.service.scores(orgId, id);
    if (!scores) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'trace_run not found' } });
    return reply.send(scores);
  });

  app.post('/api/traces/:id/auto-eval', async (request, reply) => {
    const { id } = request.params as { id: string };
    const orgId = orgOf(request);
    if (!orgId) {
      return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    }
    const parsed = parseBody(reply, RunAutoEvalSchema, request.body ?? {});
    if (!parsed.ok) return;
    try {
      const written = await deps.service.autoEval(orgId, id, parsed.data);
      if (written === null) {
        return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'trace_run not found' } });
      }
      return reply.send({ traceRunId: id, written });
    } catch (err) {
      return reply.code(404).send({
        error: { code: 'AUTO_EVAL_FAILED', message: (err as Error).message },
      });
    }
  });

  app.get('/api/scores/summary', async (request, reply) => {
    const orgId = orgOf(request);
    if (!orgId) {
      return reply
        .code(401)
        .send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    }
    const parsed = parseQuery(reply, SummaryQuerySchema, request.query);
    if (!parsed.ok) return;
    const { days, evaluator } = parsed.data;
    const out = deps.service.summary(orgId, {
      days,
      ...(evaluator ? { evaluator } : {}),
    });
    return reply.send({ orgId, days, ...out });
  });
}
