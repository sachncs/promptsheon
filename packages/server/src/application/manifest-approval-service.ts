import type { Manifest } from '@promptsheon/shared';

/** A persisted maker-checker vote returned by the approval workflow. */
export interface ManifestApproval {
  userId: string;
  vote: string;
  comment: string;
  createdAt: string;
}

/** Persistence boundary required by manifest approval use cases. */
export interface ManifestApprovalStore {
  findByHash(hash: string): Manifest | null;
  upsertApproval(hash: string, userId: string, vote: 'approve' | 'reject', comment: string): boolean;
  findApprovals(hash: string): ManifestApproval[];
  countDistinctApprovers(hash: string): number;
}

/** Audit boundary required by state-changing approval use cases. */
export interface ManifestApprovalAudit {
  append(entry: {
    userId: string;
    action: string;
    resource: string;
    details: string;
    resourceKind: string;
    resourceId: string;
  }): unknown;
}

/** Result of reading or recording a manifest approval decision. */
export interface ManifestApprovalSummary {
  hash: string;
  approvals: ManifestApproval[];
  distinctApprovers: number;
}

/** Coordinates manifest existence, voting, persistence, and audit recording. */
export class ManifestApprovalService {
  constructor(
    private readonly store: ManifestApprovalStore,
    private readonly audit: ManifestApprovalAudit,
  ) {}

  get(hash: string): ManifestApprovalSummary | null {
    if (!this.store.findByHash(hash)) return null;
    return this.summary(hash);
  }

  vote(hash: string, userId: string, vote: 'approve' | 'reject', comment: string): ManifestApprovalSummary | null {
    if (!this.store.findByHash(hash)) return null;

    this.store.upsertApproval(hash, userId, vote, comment);
    this.audit.append({
      userId,
      action: `manifest.${vote}`,
      resource: 'manifest',
      details: JSON.stringify({ manifestHash: hash, comment }),
      resourceKind: 'manifest',
      resourceId: hash,
    });
    return this.summary(hash);
  }

  private summary(hash: string): ManifestApprovalSummary {
    return {
      hash,
      approvals: this.store.findApprovals(hash),
      distinctApprovers: this.store.countDistinctApprovers(hash),
    };
  }
}
