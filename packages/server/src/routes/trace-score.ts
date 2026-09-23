import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type { TraceRepo } from '../repos/trace.js';
import type { TraceScoreRepo } from '../repos/trace-score.js';
import type { AutoEval } from '../observability/auto-eval.js';
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
  deps: { traceRepo: TraceRepo; scoreRepo: TraceScoreRepo; autoEval: AutoEval },
) {
  app.get('/api/traces/:id/scores', async (request, reply) => {
    const { id } = request.params as { id: string };
    const orgId = orgOf(request);
    if (!orgId) {
      return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    }
    const run = deps.traceRepo.findByIdInOrg(id, orgId);
    if (!run) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'trace_run not found' } });
    const parsed = parseQuery(reply, ListScoresQuerySchema, request.query);
    if (!parsed.ok) return;
    const items = deps.scoreRepo.listByRun(id);
    return reply.send({ run, items, total: items.length });
  });

  app.post('/api/traces/:id/auto-eval', async (request, reply) => {
    const { id } = request.params as { id: string };
    const orgId = orgOf(request);
    if (!orgId) {
      return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    }
    if (!deps.traceRepo.findByIdInOrg(id, orgId)) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'trace_run not found' } });
    }
    const parsed = parseBody(reply, RunAutoEvalSchema, request.body ?? {});
    if (!parsed.ok) return;
    try {
      const written = await deps.autoEval.run(id, parsed.data);
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
    const out = deps.scoreRepo.summaryByOrg(orgId, {
      days,
      ...(evaluator ? { evaluator } : {}),
    });
    return reply.send({ orgId, days, ...out });
  });
}
