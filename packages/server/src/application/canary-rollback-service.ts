import type { EvalRepo } from '../repos/eval.js';
import type { ReleaseRepo } from '../repos/release.js';
import type { AuditChain } from '../audit/chain.js';

export interface CanaryAssessment {
  action: 'no_action' | 'rolled_back';
  reason: string;
  releaseId: string;
  targetReleaseId?: string;
  evaluationId?: string;
}

/** Applies a deterministic quality floor to canary/active releases. */
export class CanaryRollbackService {
  constructor(
    private readonly releases: ReleaseRepo,
    private readonly evaluations: EvalRepo,
    private readonly audit: AuditChain,
    private readonly minimumScore = 0.7,
  ) {}

  assess(releaseId: string, organizationId: string, actorId: string): CanaryAssessment {
    const release = this.releases.findByIdInOrg(releaseId, organizationId);
    if (!release) return { action: 'no_action', reason: 'release not found', releaseId };
    if (release.status !== 'canary' && release.status !== 'active') {
      return { action: 'no_action', reason: `release is ${release.status}`, releaseId };
    }
    const evaluation = this.evaluations.findRunsByReleaseIdInOrg(releaseId, organizationId)[0];
    if (!evaluation) return { action: 'no_action', reason: 'no evaluation result is available', releaseId };
    if (evaluation.status === 'passed' && evaluation.score >= this.minimumScore) {
      return { action: 'no_action', reason: 'evaluation is healthy', releaseId, evaluationId: evaluation.id };
    }
    const target = this.releases.findActivePeerInOrg(release, organizationId);
    if (!target) return { action: 'no_action', reason: 'regression detected but no active rollback target exists', releaseId, evaluationId: evaluation.id };
    const result = this.releases.rollbackAtomicallyInOrg(release.id, target.id, organizationId);
    if (!result) return { action: 'no_action', reason: 'rollback transaction failed', releaseId, targetReleaseId: target.id, evaluationId: evaluation.id };
    this.audit.append({
      userId: actorId,
      action: 'release.auto_rollback',
      resource: 'release',
      details: JSON.stringify({ releaseId, targetReleaseId: target.id, evaluationId: evaluation.id, score: evaluation.score }),
      resourceKind: 'release',
      resourceId: releaseId,
    });
    return { action: 'rolled_back', reason: 'evaluation quality regression exceeded the configured floor', releaseId, targetReleaseId: target.id, evaluationId: evaluation.id };
  }
}
