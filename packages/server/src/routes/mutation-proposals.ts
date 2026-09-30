import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { parseBody, parseParams, parseQuery } from './validate.js';
import type { MutationProposalRepo } from '../repos/mutation-proposal.js';
import type { MutationPromotionService } from '../application/mutation-promotion-service.js';
import type { MutationEvaluationService } from '../application/mutation-evaluation-service.js';
import { registerFallbackRouteDoc } from '../openapi.js';

const MutationKindSchema = z.enum(['prompt', 'guardrail', 'model', 'routing', 'context', 'tool', 'permission', 'execution', 'memory', 'budget']);
const ProposalStatusSchema = z.enum(['proposed', 'validated', 'approved', 'rejected', 'abandoned']);
const RiskSchema = z.enum(['low', 'medium', 'high', 'critical']);
const AuthorTypeSchema = z.enum(['human', 'system', 'simulator']);

const CreateSchema = z.object({
  sourceHash: z.string().trim().min(1).max(255),
  candidateHash: z.string().trim().min(1).max(255).optional(),
  mutationKind: MutationKindSchema,
  changes: z.record(z.string(), z.unknown()),
  rationale: z.string().trim().min(1).max(10_000),
  expectedOutcome: z.string().trim().min(1).max(10_000),
  authorType: AuthorTypeSchema.default('human'),
  risk: RiskSchema,
  confidence: z.number().min(0).max(1),
  evaluationRunId: z.string().trim().min(1).max(255).optional(),
});

const ListQuerySchema = z.object({
  status: ProposalStatusSchema.optional(),
  sourceHash: z.string().trim().min(1).max(255).optional(),
});
const IdParamsSchema = z.object({ id: z.string().trim().min(1).max(255) });
const DecideSchema = z.object({
  decision: z.enum(['approve', 'reject', 'abandon']),
  reason: z.string().trim().min(1).max(4000),
});
const PromoteSchema = z.object({ environment: z.enum(['dev', 'staging', 'prod']).default('dev') });
const EvaluationSchema = z.object({
  evaluationRunId: z.string().trim().min(1).max(255),
  baselineScore: z.number().min(0).max(1),
});

interface OrgRequest {
  orgContext?: { orgId?: string };
  agentOrgId?: string;
}

function organizationIdOf(request: OrgRequest): string | null {
  return request.orgContext?.orgId ?? request.agentOrgId ?? null;
}

export interface MutationProposalDeps {
  mutationProposalRepo: MutationProposalRepo;
  promotionService: MutationPromotionService;
  evaluationService: MutationEvaluationService;
  actorId: (request: OrgRequest) => string;
}

export function registerMutationProposalRoutes(app: FastifyInstance, deps: MutationProposalDeps): void {
  registerFallbackRouteDoc('get', '/api/mutation-proposals');
  registerFallbackRouteDoc('post', '/api/mutation-proposals');
  registerFallbackRouteDoc('get', '/api/mutation-proposals/:id');
  registerFallbackRouteDoc('post', '/api/mutation-proposals/:id/validate');
  registerFallbackRouteDoc('post', '/api/mutation-proposals/:id/evaluation');
  registerFallbackRouteDoc('post', '/api/mutation-proposals/:id/decision');
  registerFallbackRouteDoc('post', '/api/mutation-proposals/:id/promote');

  app.get('/api/mutation-proposals', async (request, reply) => {
    const organizationId = organizationIdOf(request);
    if (!organizationId) return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    const parsed = parseQuery(reply, ListQuerySchema, request.query);
    if (!parsed.ok) return;
    return reply.send(deps.mutationProposalRepo.listInOrg(organizationId, parsed.data));
  });

  app.post('/api/mutation-proposals', async (request, reply) => {
    const organizationId = organizationIdOf(request);
    if (!organizationId) return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    const parsed = parseBody(reply, CreateSchema, request.body);
    if (!parsed.ok) return;
    const proposal = deps.mutationProposalRepo.create({
      ...parsed.data,
      organizationId,
      authorId: deps.actorId(request),
    });
    return reply.code(201).send(proposal);
  });

  app.get('/api/mutation-proposals/:id', async (request, reply) => {
    const organizationId = organizationIdOf(request);
    if (!organizationId) return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    const parsed = parseParams(reply, IdParamsSchema, request.params);
    if (!parsed.ok) return;
    const proposal = deps.mutationProposalRepo.findInOrg(parsed.data.id, organizationId);
    if (!proposal) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'mutation proposal not found' } });
    return reply.send(proposal);
  });

  app.post('/api/mutation-proposals/:id/validate', async (request, reply) => {
    const organizationId = organizationIdOf(request);
    if (!organizationId) return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    const parsedParams = parseParams(reply, IdParamsSchema, request.params);
    if (!parsedParams.ok) return;
    try {
      const proposal = await deps.promotionService.validate({
        proposalId: parsedParams.data.id,
        organizationId,
      });
      return reply.send(proposal);
    } catch (error) {
      request.log.warn({ err: error }, 'mutation proposal validation rejected');
      const message = error instanceof Error ? error.message : 'mutation proposal validation failed';
      return reply.code(422).send({ error: { code: 'VALIDATION_REJECTED', message } });
    }
  });

  app.post('/api/mutation-proposals/:id/evaluation', async (request, reply) => {
    const organizationId = organizationIdOf(request);
    if (!organizationId) return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    const parsedParams = parseParams(reply, IdParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const parsed = parseBody(reply, EvaluationSchema, request.body);
    if (!parsed.ok) return;
    const result = deps.evaluationService.attach({
      proposalId: parsedParams.data.id,
      organizationId,
      ...parsed.data,
    });
    if (result.kind === 'success') return reply.send(result.proposal);
    const errors: Record<Exclude<typeof result.kind, 'success'>, { code: string; message: string; status: number }> = {
      'proposal-not-found': { code: 'NOT_FOUND', message: 'mutation proposal not found', status: 404 },
      'candidate-required': { code: 'CANDIDATE_REQUIRED', message: 'an immutable candidate must be materialised before evaluation evidence can be attached', status: 422 },
      'proposal-not-pending': { code: 'EVALUATION_LOCKED', message: 'evaluation evidence can only be attached while the proposal is proposed', status: 422 },
      'run-not-found': { code: 'EVIDENCE_NOT_FOUND', message: 'the evaluation run was not found in this organisation', status: 404 },
      'run-not-complete': { code: 'EVIDENCE_INCOMPLETE', message: 'the evaluation run must be completed before it can support approval', status: 422 },
      'run-failed': { code: 'EVALUATION_FAILED', message: 'the evaluation run did not pass its suite threshold', status: 422 },
      'version-not-found': { code: 'EVIDENCE_INVALID', message: 'the evaluation run references an unavailable suite version', status: 422 },
    };
    const error = errors[result.kind];
    return reply.code(error.status).send({ error: { code: error.code, message: error.message } });
  });

  app.post('/api/mutation-proposals/:id/decision', async (request, reply) => {
    const organizationId = organizationIdOf(request);
    if (!organizationId) return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    const parsedParams = parseParams(reply, IdParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const parsed = parseBody(reply, DecideSchema, request.body);
    if (!parsed.ok) return;
    const existing = deps.mutationProposalRepo.findInOrg(parsedParams.data.id, organizationId);
    if (!existing) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'mutation proposal not found' } });
    if (parsed.data.decision === 'approve' && existing.status !== 'validated') {
      return reply.code(422).send({
        error: { code: 'VALIDATION_REQUIRED', message: 'the immutable candidate must pass validation before approval' },
      });
    }
    if (parsed.data.decision === 'approve' && existing.evaluationStatus !== 'passed') {
      return reply.code(422).send({
        error: { code: 'EVALUATION_REQUIRED', message: 'the candidate must pass evaluation before approval' },
      });
    }
    if (parsed.data.decision === 'approve' && !existing.evaluationRunId) {
      return reply.code(422).send({
        error: { code: 'EVIDENCE_REQUIRED', message: 'a durable evaluation evidence reference is required before approval' },
      });
    }
    if (parsed.data.decision === 'approve' && !existing.candidateHash) {
      return reply.code(422).send({
        error: { code: 'CANDIDATE_REQUIRED', message: 'an immutable candidate must be materialised before approval' },
      });
    }
    const status = parsed.data.decision === 'approve' ? 'approved' : parsed.data.decision === 'reject' ? 'rejected' : 'abandoned';
    const proposal = deps.mutationProposalRepo.decide({
      id: parsedParams.data.id,
      organizationId,
      status,
      reviewerId: deps.actorId(request),
      reason: parsed.data.reason,
    });
    if (!proposal) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'mutation proposal not found or already decided' } });
    return reply.send(proposal);
  });

  app.post('/api/mutation-proposals/:id/promote', async (request, reply) => {
    const organizationId = organizationIdOf(request);
    if (!organizationId) return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    const parsedParams = parseParams(reply, IdParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const parsed = parseBody(reply, PromoteSchema, request.body);
    if (!parsed.ok) return;
    try {
      const result = await deps.promotionService.promote({
        proposalId: parsedParams.data.id,
        organizationId,
        actorId: deps.actorId(request),
        environment: parsed.data.environment,
      });
      return reply.code(201).send(result);
    } catch (error) {
      request.log.warn({ err: error }, 'mutation proposal promotion rejected');
      const message = error instanceof Error ? error.message : 'mutation proposal promotion failed';
      return reply.code(422).send({ error: { code: 'PROMOTION_REJECTED', message } });
    }
  });
}
