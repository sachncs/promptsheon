import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { parseBody } from './validate.js';
import type { ExperimentRepo } from '../repos/experiment.js';

const VariantSchema = z.object({
  label: z.string().min(1).max(120),
  config: z.record(z.string(), z.unknown()),
  weight: z.number().min(0).max(1).optional(),
});

const AssignmentSchema = z.object({
  caseId: z.string(),
  variantId: z.string(),
  outcome: z.enum(['pass', 'fail', 'borderline', 'error']),
});

export interface ExperimentDeps {
  experimentRepo: ExperimentRepo;
}

export function registerExperimentRoutes(app: FastifyInstance, deps: ExperimentDeps): void {
  app.get('/api/releases/:id/experiments', async (request, reply) => {
    const { id } = request.params as { id: string };
    return reply.send({
      variants: deps.experimentRepo.listVariants(id),
      assignments: deps.experimentRepo.listVariants(id).flatMap((v) =>
        deps.experimentRepo.listAssignments(v.id).map((a) => ({ ...a, variantLabel: v.label })),
      ),
      compare: deps.experimentRepo.compare(id),
    });
  });

  app.post('/api/releases/:id/experiments', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, VariantSchema, request.body);
    if (!parsed.ok) return;
    const variant = deps.experimentRepo.createVariant({
      releaseId: id,
      label: parsed.data.label,
      config: JSON.stringify(parsed.data.config),
      weight: parsed.data.weight ?? 0.5,
    });
    return reply.code(201).send(variant);
  });

  app.post('/api/experiments/:variantId/assignments', async (request, reply) => {
    const { variantId } = request.params as { variantId: string };
    const parsed = parseBody(reply, AssignmentSchema, request.body);
    if (!parsed.ok) return;
    const a = deps.experimentRepo.recordAssignment({
      experimentId: variantId,
      caseId: parsed.data.caseId,
      variantId,
      outcome: parsed.data.outcome,
    });
    return reply.code(201).send(a);
  });
}
