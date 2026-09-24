import type { FastifyInstance, FastifyRequest } from 'fastify';
import { z } from 'zod';
import type { UserAnalyticsRepo } from '../repos/user-analytics.js';
import { parseQuery } from './validate.js';

const UserAnalyticsDaysQuerySchema = z.object({
  days: z.coerce.number().int().min(1).max(365).default(30),
});

const OrgAnalyticsQuerySchema = z.object({
  days: z.coerce.number().int().min(1).max(365).default(30),
  limit: z.coerce.number().int().min(1).max(200).default(25),
});

const UserAnalyticsPathSchema = z.object({
  userId: z.string().min(1).max(120),
});

function orgOf(request: FastifyRequest): string | null {
  return request.orgContext?.orgId ?? request.agentOrgId ?? null;
}

/**
 * GET  /api/analytics/users/:userId?days= — per-user daily usage.
 * GET  /api/analytics/leaderboard?days=&limit= — top consumers
 *   in the active org (admin-gated downstream).
 * GET  /api/analytics/org-totals?days= — org-wide totals
 *   (tokens, cost, runs, active days).
 */
export function registerAnalyticsRoutes(
  app: FastifyInstance,
  deps: { repo: UserAnalyticsRepo },
) {
  app.get('/api/analytics/users/:userId', async (request, reply) => {
    const organizationId = orgOf(request);
    if (!organizationId) {
      return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    }
    const parsedPath = UserAnalyticsPathSchema.safeParse(request.params);
    if (!parsedPath.success) {
      return reply.code(422).send({ error: { code: 'VALIDATION_ERROR', message: 'Path validation failed' } });
    }
    const parsed = parseQuery(reply, UserAnalyticsDaysQuerySchema, request.query);
    if (!parsed.ok) return;
    const { userId } = parsedPath.data;
    const { days } = parsed.data;
    const perDay = deps.repo.perDay(userId, organizationId, days);
    return reply.send({ userId, days, perDay });
  });

  app.get('/api/analytics/leaderboard', async (request, reply) => {
    const orgId = orgOf(request);
    if (!orgId) {
      return reply
        .code(401)
        .send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    }
    const parsed = parseQuery(reply, OrgAnalyticsQuerySchema, request.query);
    if (!parsed.ok) return;
    const items = deps.repo.leaderboardByOrg(orgId, parsed.data);
    return reply.send({ orgId, ...parsed.data, items });
  });

  app.get('/api/analytics/org-totals', async (request, reply) => {
    const orgId = orgOf(request);
    if (!orgId) {
      return reply
        .code(401)
        .send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    }
    const parsed = parseQuery(reply, OrgAnalyticsQuerySchema, request.query);
    if (!parsed.ok) return;
    const totals = deps.repo.orgTotals(orgId, parsed.data.days);
    return reply.send({ orgId, ...parsed.data, totals });
  });
}
