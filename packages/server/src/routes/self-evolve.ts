import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type { EvolutionAgent } from '../agents/evolution/evolution.js';
import type { CapabilityRepo } from '../repos/capability.js';
import type { EvalRepo } from '../repos/eval.js';
import { parseBody, parseParams } from './validate.js';

const RunCycleSchema = z.object({
  capabilityId: z.string().min(1),
});
const CapabilityParamsSchema = z.object({ capabilityId: z.string().trim().min(1).max(255) });

interface RequestOrganizationContext {
  agentOrgId?: string;
  orgContext?: { orgId?: string; organizationId?: string };
}

function requireOrganization(request: unknown, reply: { code: (status: number) => { send: (body: unknown) => unknown } }): string | null {
  const context = (request as RequestOrganizationContext | undefined) ?? {};
  const organizationId = context.orgContext?.orgId ?? context.orgContext?.organizationId ?? context.agentOrgId;
  if (organizationId) return organizationId;
  void reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
  return null;
}

export function registerSelfEvolveRoutes(
  app: FastifyInstance,
  evolutionAgent: EvolutionAgent,
  capabilityRepo: CapabilityRepo,
  _evalRepo: EvalRepo,
) {
  app.post('/api/self-evolve/run', async (request, reply) => {
    const parsed = parseBody(reply, RunCycleSchema, request.body);
    if (!parsed.ok) return;
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const { capabilityId } = parsed.data;
    const capability = capabilityRepo.findByIdInOrg(capabilityId, organizationId);
    if (!capability) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Capability not found' } });
    }

    const manifestHash = capability.selfEvolveTargetEnv;

    const result = await evolutionAgent.runCycle(capabilityId, manifestHash, [], capability);
    return reply.send(result);
  });

  app.get('/api/self-evolve/:capabilityId/state', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsedParams = parseParams(reply, CapabilityParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { capabilityId } = parsedParams.data;
    if (!capabilityRepo.findByIdInOrg(capabilityId, organizationId)) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Capability not found' } });
    }
    const state = evolutionAgent.getState(capabilityId);
    return reply.send(state ?? { status: 'idle', cycleCount: 0 });
  });

  /**
   * Capability-scoped shortcuts the frontend uses:
   *   GET  /api/capabilities/:capabilityId/self-evolve          → state
   *   POST /api/capabilities/:capabilityId/self-evolve/run      → cycle
   */
  app.get('/api/capabilities/:capabilityId/self-evolve', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsedParams = parseParams(reply, CapabilityParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { capabilityId } = parsedParams.data;
    if (!capabilityRepo.findByIdInOrg(capabilityId, organizationId)) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Capability not found' } });
    }
    const state = evolutionAgent.getState(capabilityId);
    return reply.send(state ?? { status: 'idle', cycleCount: 0 });
  });

  app.post('/api/capabilities/:capabilityId/self-evolve/run', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    const parsedParams = parseParams(reply, CapabilityParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { capabilityId } = parsedParams.data;
    if (!organizationId) return;
    const capability = capabilityRepo.findByIdInOrg(capabilityId, organizationId);
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
