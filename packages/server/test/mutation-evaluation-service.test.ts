import { describe, expect, it, vi } from 'vitest';
import type { EvalSuiteRun, MutationProposal } from '@promptsheon/shared';
import { MutationEvaluationService, type MutationEvaluationEvidenceStore } from '../src/application/mutation-evaluation-service.js';
import type { MutationProposalRepo } from '../src/repos/mutation-proposal.js';

const proposal = {
  id: 'proposal-1',
  organizationId: 'org-1',
  candidateHash: 'candidate-1',
  status: 'proposed',
} as MutationProposal;

const run = {
  id: 'run-1',
  suiteVersionId: 'version-1',
  status: 'completed',
  passed: true,
  rawScore: 0.93,
} as unknown as EvalSuiteRun;

describe('MutationEvaluationService', () => {
  it('attaches a completed passing run and uses the suite threshold', () => {
    const proposals = {
      findInOrg: vi.fn(() => proposal),
      recordEvaluation: vi.fn(() => ({ ...proposal, evaluationStatus: 'passed', evaluationRunId: 'run-1' })),
    } as unknown as MutationProposalRepo;
    const evidence = {
      findRunInOrg: vi.fn(() => run),
      findVersionByIdInOrg: vi.fn(() => ({ passThreshold: 0.9 })),
    } as unknown as MutationEvaluationEvidenceStore;
    const service = new MutationEvaluationService(proposals, evidence);

    const result = service.attach({
      proposalId: 'proposal-1',
      organizationId: 'org-1',
      evaluationRunId: 'run-1',
      baselineScore: 0.8,
    });

    expect(result).toMatchObject({ kind: 'success' });
    expect(proposals.recordEvaluation).toHaveBeenCalledWith({
      id: 'proposal-1',
      organizationId: 'org-1',
      baselineScore: 0.8,
      candidateScore: 0.93,
      passThreshold: 0.9,
      evaluationRunId: 'run-1',
    });
  });

  it('fails closed when the durable run did not pass', () => {
    const proposals = { findInOrg: vi.fn(() => proposal), recordEvaluation: vi.fn() } as unknown as MutationProposalRepo;
    const evidence = {
      findRunInOrg: vi.fn(() => ({ ...run, passed: false })),
      findVersionByIdInOrg: vi.fn(),
    } as unknown as MutationEvaluationEvidenceStore;
    const service = new MutationEvaluationService(proposals, evidence);

    expect(service.attach({ proposalId: 'proposal-1', organizationId: 'org-1', evaluationRunId: 'run-1', baselineScore: 0.8 })).toEqual({ kind: 'run-failed' });
    expect(proposals.recordEvaluation).not.toHaveBeenCalled();
  });

  it('does not report success when a concurrent state change prevents attachment', () => {
    const proposals = {
      findInOrg: vi.fn(() => proposal),
      recordEvaluation: vi.fn(() => ({ ...proposal, evaluationStatus: 'pending', evaluationRunId: null })),
    } as unknown as MutationProposalRepo;
    const evidence = {
      findRunInOrg: vi.fn(() => run),
      findVersionByIdInOrg: vi.fn(() => ({ passThreshold: 0.9 })),
    } as unknown as MutationEvaluationEvidenceStore;
    const service = new MutationEvaluationService(proposals, evidence);

    expect(service.attach({ proposalId: 'proposal-1', organizationId: 'org-1', evaluationRunId: 'run-1', baselineScore: 0.8 })).toEqual({ kind: 'proposal-not-pending' });
  });
});
