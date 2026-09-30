import type { MutationProposal, EvalSuiteRun, EvalSuiteVersion } from '@promptsheon/shared';
import type { MutationProposalRepo } from '../repos/mutation-proposal.js';

/** Minimal read port for durable suite evidence used by proposal review. */
export interface MutationEvaluationEvidenceStore {
  findRunInOrg(id: string, organizationId: string): EvalSuiteRun | null;
  findVersionByIdInOrg(id: string, organizationId: string): EvalSuiteVersion | null;
}

export type MutationEvaluationAttachResult =
  | { kind: 'proposal-not-found' }
  | { kind: 'candidate-required' }
  | { kind: 'proposal-not-pending' }
  | { kind: 'run-not-found' }
  | { kind: 'run-not-complete' }
  | { kind: 'run-failed' }
  | { kind: 'version-not-found' }
  | { kind: 'success'; proposal: MutationProposal };

/** Attaches a tenant-scoped, completed suite run to a mutation proposal. */
export class MutationEvaluationService {
  constructor(
    private readonly proposals: MutationProposalRepo,
    private readonly evidence: MutationEvaluationEvidenceStore,
  ) {}

  attach(input: {
    proposalId: string;
    organizationId: string;
    evaluationRunId: string;
    baselineScore: number;
  }): MutationEvaluationAttachResult {
    const proposal = this.proposals.findInOrg(input.proposalId, input.organizationId);
    if (!proposal) return { kind: 'proposal-not-found' };
    if (!proposal.candidateHash) return { kind: 'candidate-required' };
    if (proposal.status !== 'proposed') return { kind: 'proposal-not-pending' };

    const run = this.evidence.findRunInOrg(input.evaluationRunId, input.organizationId);
    if (!run) return { kind: 'run-not-found' };
    if (run.status !== 'completed') return { kind: 'run-not-complete' };
    if (!run.passed) return { kind: 'run-failed' };

    const version = this.evidence.findVersionByIdInOrg(run.suiteVersionId, input.organizationId);
    if (!version) return { kind: 'version-not-found' };

    const updated = this.proposals.recordEvaluation({
      id: proposal.id,
      organizationId: input.organizationId,
      baselineScore: input.baselineScore,
      candidateScore: run.rawScore,
      passThreshold: version.passThreshold,
      evaluationRunId: run.id,
    });
    if (!updated) return { kind: 'proposal-not-found' };
    if (updated.evaluationRunId !== run.id) return { kind: 'proposal-not-pending' };
    return { kind: 'success', proposal: updated };
  }
}
