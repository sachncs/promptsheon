import type { FastifyInstance } from 'fastify';
import type { EvolutionAgent } from '../agents/evolution/evolution.js';
import type { CapabilityRepo } from '../repos/capability.js';
import type { EvalRepo } from '../repos/eval.js';

export function registerSelfEvolveRoutes(
  app: FastifyInstance,
  evolutionAgent: EvolutionAgent,
  capabilityRepo: CapabilityRepo,
  _evalRepo: EvalRepo,
) {
  app.post('/api/self-evolve/run', async (request, reply) => {
    const { capabilityId } = request.body as { capabilityId: string };
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
}
