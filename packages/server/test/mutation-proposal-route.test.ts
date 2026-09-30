import Fastify, { type FastifyInstance } from 'fastify';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { MutationProposal } from '@promptsheon/shared';
import { registerMutationProposalRoutes } from '../src/routes/mutation-proposals.js';
import type { MutationProposalRepo } from '../src/repos/mutation-proposal.js';
import type { MutationPromotionService } from '../src/application/mutation-promotion-service.js';
import type { MutationEvaluationService } from '../src/application/mutation-evaluation-service.js';

const proposal = {
  id: 'proposal-1',
  organizationId: 'org-1',
  sourceHash: 'source-hash',
  candidateHash: 'candidate-hash',
  mutationKind: 'prompt',
  changes: {},
  rationale: 'Improve clarity',
  expectedOutcome: 'Higher task success',
  authorType: 'simulator',
  authorId: 'simulator',
  risk: 'low',
  confidence: 0.8,
  baselineScore: 0.2,
  candidateScore: 0.8,
  evaluationStatus: 'pending',
  status: 'proposed',
  evaluationRunId: null,
  decisionReason: null,
  reviewedBy: null,
  reviewedAt: null,
  createdAt: '2026-01-01T00:00:00.000Z',
  updatedAt: '2026-01-01T00:00:00.000Z',
  promotedReleaseId: null,
  promotedAt: null,
} satisfies MutationProposal;

function buildApp(): { app: FastifyInstance; repo: MutationProposalRepo; promotion: MutationPromotionService; evaluation: MutationEvaluationService } {
  const app = Fastify();
  app.addHook('onRequest', async (request) => {
    request.agentOrgId = 'org-1';
  });
  const repo = {
    findInOrg: vi.fn(() => proposal),
    listInOrg: vi.fn(() => [proposal]),
    create: vi.fn(),
    decide: vi.fn(),
  } as unknown as MutationProposalRepo;
  const promotion = {
    validate: vi.fn(async () => ({ ...proposal, status: 'validated' as const })),
    promote: vi.fn(),
  } as unknown as MutationPromotionService;
  const evaluation = {
    attach: vi.fn(() => ({ kind: 'success' as const, proposal: { ...proposal, evaluationStatus: 'passed' as const, evaluationRunId: 'run-1' } })),
  } as unknown as MutationEvaluationService;
  registerMutationProposalRoutes(app, { mutationProposalRepo: repo, promotionService: promotion, evaluationService: evaluation, actorId: () => 'reviewer' });
  return { app, repo, promotion, evaluation };
}

describe('mutation proposal routes', () => {
  let app: FastifyInstance | undefined;

  afterEach(async () => {
    await app?.close();
    app = undefined;
  });

  it('validates a candidate through the application service', async () => {
    const context = buildApp();
    app = context.app;
    const response = await app.inject({ method: 'POST', url: '/api/mutation-proposals/proposal-1/validate' });

    expect(response.statusCode).toBe(200);
    expect(response.json()).toMatchObject({ id: 'proposal-1', status: 'validated' });
    expect(context.promotion.validate).toHaveBeenCalledWith({ proposalId: 'proposal-1', organizationId: 'org-1' });
  });

  it('blocks approval until the candidate is validated', async () => {
    const context = buildApp();
    app = context.app;
    const response = await app.inject({
      method: 'POST',
      url: '/api/mutation-proposals/proposal-1/decision',
      payload: { decision: 'approve', reason: 'ship it' },
    });

    expect(response.statusCode).toBe(422);
    expect(response.json()).toMatchObject({ error: { code: 'VALIDATION_REQUIRED' } });
    expect(context.repo.decide).not.toHaveBeenCalled();
  });

  it('attaches completed evaluation evidence through the application service', async () => {
    const context = buildApp();
    app = context.app;
    const response = await app.inject({
      method: 'POST',
      url: '/api/mutation-proposals/proposal-1/evaluation',
      payload: { evaluationRunId: 'run-1', baselineScore: 0.7 },
    });

    expect(response.statusCode).toBe(200);
    expect(response.json()).toMatchObject({ evaluationStatus: 'passed', evaluationRunId: 'run-1' });
    expect(context.evaluation.attach).toHaveBeenCalledWith({
      proposalId: 'proposal-1',
      organizationId: 'org-1',
      evaluationRunId: 'run-1',
      baselineScore: 0.7,
    });
  });

  it('blocks approval when validation exists but evaluation has not passed', async () => {
    const context = buildApp();
    app = context.app;
    vi.mocked(context.repo.findInOrg).mockReturnValue({ ...proposal, status: 'validated' });
    const response = await app.inject({
      method: 'POST',
      url: '/api/mutation-proposals/proposal-1/decision',
      payload: { decision: 'approve', reason: 'ship it' },
    });

    expect(response.statusCode).toBe(422);
    expect(response.json()).toMatchObject({ error: { code: 'EVALUATION_REQUIRED' } });
    expect(context.repo.decide).not.toHaveBeenCalled();
  });

  it('blocks approval when a passed evaluation has no durable evidence reference', async () => {
    const context = buildApp();
    app = context.app;
    vi.mocked(context.repo.findInOrg).mockReturnValue({ ...proposal, status: 'validated', evaluationStatus: 'passed' });
    const response = await app.inject({
      method: 'POST',
      url: '/api/mutation-proposals/proposal-1/decision',
      payload: { decision: 'approve', reason: 'ship it' },
    });

    expect(response.statusCode).toBe(422);
    expect(response.json()).toMatchObject({ error: { code: 'EVIDENCE_REQUIRED' } });
    expect(context.repo.decide).not.toHaveBeenCalled();
  });
});
