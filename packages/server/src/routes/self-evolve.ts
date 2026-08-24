import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type { EvolutionAgent } from '../agents/evolution/evolution.js';
import type { CapabilityRepo } from '../repos/capability.js';
import type { EvalRepo } from '../repos/eval.js';
import { parseBody } from './validate.js';

const RunCycleSchema = z.object({
  capabilityId: z.string().min(1),
});

export function registerSelfEvolveRoutes(
  app: FastifyInstance,
  evolutionAgent: EvolutionAgent,
  capabilityRepo: CapabilityRepo,
  _evalRepo: EvalRepo,
) {
  app.post('/api/self-evolve/run', async (request, reply) => {
    const parsed = parseBody(reply, RunCycleSchema, request.body);
    if (!parsed.ok) return;
    const { capabilityId } = parsed.data;
    const capability = capabilityRepo.findById(capabilityId);
    if (!capability) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Capability not found' } });
    }

    const manifestHash = capability.selfEvolveTargetEnv;

    const result = await evolutionAgent.runCycle(capabilityId, manifestHash, [], capability);
    return reply.send(result);
  });

  app.get('/api/self-evolve/:capabilityId/state', async (request, reply) => {
    const { capabilityId } = request.params as { capabilityId: string };
    const state = evolutionAgent.getState(capabilityId);
    return reply.send(state ?? { status: 'idle', cycleCount: 0 });
  });

  /**
   * Capability-scoped shortcuts the frontend uses:
   *   GET  /api/capabilities/:capabilityId/self-evolve          → state
   *   POST /api/capabilities/:capabilityId/self-evolve/run      → cycle
   */
  app.get('/api/capabilities/:capabilityId/self-evolve', async (request, reply) => {
    const { capabilityId } = request.params as { capabilityId: string };
    const state = evolutionAgent.getState(capabilityId);
    return reply.send(state ?? { status: 'idle', cycleCount: 0 });
  });

  app.post('/api/capabilities/:capabilityId/self-evolve/run', async (request, reply) => {
    const { capabilityId } = request.params as { capabilityId: string };
    const capability = capabilityRepo.findById(capabilityId);
    if (!capability) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Capability not found' } });
    }
    const result = await evolutionAgent.runCycle(
      capabilityId,
      capability.selfEvolveTargetEnv,
      [],
      capability,
    );
    return reply.send(result);
  });
}