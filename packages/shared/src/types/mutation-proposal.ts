export type MutationKind =
  | 'prompt'
  | 'guardrail'
  | 'model'
  | 'routing'
  | 'context'
  | 'tool'
  | 'permission'
  | 'execution'
  | 'memory'
  | 'budget';

export type MutationAuthorType = 'human' | 'system' | 'simulator';
export type MutationRisk = 'low' | 'medium' | 'high' | 'critical';
export type MutationProposalStatus = 'proposed' | 'validated' | 'approved' | 'rejected' | 'abandoned';
export type MutationEvaluationStatus = 'pending' | 'passed' | 'failed';

export interface MutationProposal {
  id: string;
  organizationId: string;
  sourceHash: string;
  candidateHash: string | null;
  mutationKind: MutationKind;
  changes: Record<string, unknown>;
  rationale: string;
  expectedOutcome: string;
  authorType: MutationAuthorType;
  authorId: string;
  risk: MutationRisk;
  confidence: number;
  baselineScore: number | null;
  candidateScore: number | null;
  evaluationStatus: MutationEvaluationStatus;
  status: MutationProposalStatus;
  evaluationRunId: string | null;
  decisionReason: string | null;
  reviewedBy: string | null;
  reviewedAt: string | null;
  createdAt: string;
  updatedAt: string;
  promotedReleaseId: string | null;
  promotedAt: string | null;
}
