import type { FastifyInstance } from 'fastify';
import type { GoalBasedEvolutionAgent, EvolutionSnapshot } from '../agents/evolution/goal-evolver.js';
import { parseQuery } from './validate.js';
import { z } from 'zod';

const ListQuerySchema = z.object({
  limit: z.coerce.number().int().min(1).max(100).optional().default(20),
});

export interface GoalSummary {
  manifestHash: string;
  bestScore: number;
  iterations: number;
  lastUpdated: string;
}

export interface GoalDetail {
  manifestHash: string;
  bestScore: number;
  bestManifestHash: string;
  iterations: number;
  totalCost: number;
  snapshots: EvolutionSnapshot[];
  history: Array<{ iteration: number; score: number; cost: number; revised: boolean; timestamp: string }>;
}

/**
 * Goal observability routes.
 * - GET /api/goals: list of in-progress / recent goal runs
 * - GET /api/goals/:hash: drill-down with iteration history + snapshots
 *
 * Note: the agent's run state is in-memory. For production this would
 * be backed by ManifestRepo state. v1 reads from agent.getState()
 * (single-process only) and reconstructs summary from the manifest DAG.
 */
export function registerGoalObservabilityRoutes(
  app: FastifyInstance,
  deps: { goalEvolver: GoalBasedEvolutionAgent; getActiveGoals: () => GoalSummary[] },
) {
  app.get('/api/goals', async (request, reply) => {
    parseQuery(reply, ListQuerySchema, request.query);
    return reply.send({ goals: deps.getActiveGoals() });
  });

  app.get('/api/goals/:hash', async (request, reply) => {
    const { hash } = request.params as { hash: string };
    const state = deps.goalEvolver.getState(hash);
    if (!state) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Goal state not found' } });
    }
    const detail: GoalDetail = {
      manifestHash: state.currentHash,
      bestScore: state.bestScore,
      bestManifestHash: state.bestHash,
      iterations: state.iteration,
      totalCost: 0,
      snapshots: [],
      history: [],
    };
    return reply.send(detail);
  });
}