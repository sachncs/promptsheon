import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type { IdeaPlannerAgent } from '../agents/planner/index.js';
import { parseBody } from './validate.js';

export const PlanIdeaSchema = z.object({
  idea: z.string().min(1).max(10000),
  constraints: z.array(z.string().max(500)).optional(),
  examples: z.array(z.object({
    input: z.unknown(),
    expected: z.unknown(),
  })).optional(),
});

export function registerIdeaRoutes(app: FastifyInstance, deps: { planner: IdeaPlannerAgent }) {
  app.post('/api/ideas/plan', async (request, reply) => {
    const parsed = parseBody(reply, PlanIdeaSchema, request.body);
    if (!parsed.ok) return;
    try {
      const result = await deps.planner.plan(parsed.data);
      return reply.send(result);
    } catch (e) {
      request.log.error({ err: e }, 'planner.plan failed');
      return reply.code(503).send({ error: { code: 'PLANNER_UNAVAILABLE', message: 'Planner failed to decompose idea' } });
    }
  });
}