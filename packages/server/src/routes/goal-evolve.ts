import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type { GoalBasedEvolutionAgent } from '../agents/evolution/goal-evolver.js';
import type { ManifestRepo } from '../repos/manifest.js';
import { parseBody } from './validate.js';
import { NotFoundError } from '@promptsheon/shared';

const EvolveSchema = z.object({
  maxIterations: z.number().int().min(1).max(20).optional().default(5),
  cooldownMs: z.number().int().min(0).max(60_000).optional().default(1000),
  costBudget: z.number().min(0).max(1000).optional().default(10),
});

export function registerGoalEvolveRoutes(
  app: FastifyInstance,
  deps: { goalEvolver: GoalBasedEvolutionAgent; manifestRepo: ManifestRepo },
) {
  app.post('/api/manifests/:hash/evolve', async (request, reply) => {
    const { hash } = request.params as { hash: string };
    const parsed = parseBody(reply, EvolveSchema, request.body);
    if (!parsed.ok) return;
    const manifest = deps.manifestRepo.findByHash(hash);
    if (!manifest) {
      throw new NotFoundError('manifest', hash);
    }
    try {
      const result = await deps.goalEvolver.evolve(hash, manifest, {
        maxIterations: parsed.data.maxIterations,
        cooldownMs: parsed.data.cooldownMs,
        costBudget: parsed.data.costBudget,
      });
      return reply.send(result);
    } catch (e) {
      request.log.error({ err: e }, 'goal evolution failed');
      return reply.code(503).send({ error: { code: 'EVOLUTION_FAILED', message: 'Goal evolution failed' } });
    }
  });
}