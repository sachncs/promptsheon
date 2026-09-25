import {
  canTransition,
  type Release,
  type ReleaseStatus,
} from '@promptsheon/shared';
import { randomUUID } from 'node:crypto';

const MIN_APPROVERS = 2;

/** Raised when a release id is not visible in the requested organization. */
export class ReleaseNotFoundError extends Error {
  constructor(releaseId: string) {
    super(`release ${releaseId} was not found`);
    this.name = 'ReleaseNotFoundError';
  }
}

/** Raised when a release state transition is not allowed by the domain. */
export class InvalidReleaseTransitionError extends Error {
  constructor(from: ReleaseStatus, to: ReleaseStatus) {
    super(`cannot transition from ${from} to ${to}`);
    this.name = 'InvalidReleaseTransitionError';
  }
}

/** Raised when a release does not satisfy the maker-checker approval gate. */
export class ReleaseApprovalRequiredError extends Error {
  constructor(reason: string) {
    super(reason);
    this.name = 'ReleaseApprovalRequiredError';
  }
}

/** Dependencies that make release transitions deterministic and testable. */
export interface ReleaseServiceDependencies {
  createId?: () => string;
  now?: () => string;
}

/** Input required to transition a release in an organization. */
export interface TransitionReleaseInput {
  releaseId: string;
  organizationId: string;
  actorId: string;
  to: ReleaseStatus;
  reason?: string;
}

/** Persistence port required by the release transition use case. */
export interface ReleaseStore {
  findByIdInOrg(releaseId: string, organizationId: string): Release | null;
  updateStatusInOrg(releaseId: string, organizationId: string, status: ReleaseStatus): Release | null;
  appendTransition(row: {
    id: string;
    releaseId: string;
    fromStatus: ReleaseStatus;
    toStatus: ReleaseStatus;
    actorId: string;
    reason: string | null;
    createdAt: string;
  }): void;
}

/** Approval lookup port required by the maker-checker gate. */
export interface ManifestApprovalStore {
  computeManifestHash(manifestJson: string): string;
  findApprovals(manifestHash: string): Array<{ userId: string }>;
}

/** Audit port required by the release transition use case. */
export interface AuditWriter {
  append(entry: {
    userId: string;
    action: string;
    resource: string;
    details: string;
    resourceKind: string;
    resourceId: string;
  }): void;
}

/**
 * Evaluate the maker-checker gate for a release manifest.
 *
 * Activation requires two distinct approvers, and the creator may not be
 * one of them. The rule is deliberately independent of HTTP concerns.
 */
export function approvalGate(
  release: Pick<Release, 'createdBy' | 'manifest'>,
  manifestRepo: ManifestApprovalStore,
): string | null {
  const manifestHash = manifestRepo.computeManifestHash(release.manifest);
  const approvers = manifestRepo.findApprovals(manifestHash);
  const distinct = new Set(approvers.map((approver) => approver.userId));
  if (distinct.has(release.createdBy)) {
    return 'creator cannot approve their own release (maker-checker)';
  }
  if (distinct.size < MIN_APPROVERS) {
    return `insufficient approvers (${distinct.size}/${MIN_APPROVERS})`;
  }
  return null;
}

/** Application use case for controlled release state transitions. */
export class ReleaseService {
  private readonly createId: () => string;
  private readonly now: () => string;

  constructor(
    private readonly repo: ReleaseStore,
    private readonly manifestRepo: ManifestApprovalStore,
    private readonly auditChain: AuditWriter,
    dependencies: ReleaseServiceDependencies = {},
  ) {
    this.createId = dependencies.createId ?? randomUUID;
    this.now = dependencies.now ?? (() => new Date().toISOString());
  }

  /** Transition a release and record its history and audit event. */
  transition(input: TransitionReleaseInput): Release {
    const existing = this.repo.findByIdInOrg(input.releaseId, input.organizationId);
    if (!existing) throw new ReleaseNotFoundError(input.releaseId);
    if (!canTransition(existing.status, input.to)) {
      throw new InvalidReleaseTransitionError(existing.status, input.to);
    }

    if (input.to === 'approved' || input.to === 'canary' || input.to === 'active') {
      const gateFailure = approvalGate(existing, this.manifestRepo);
      if (gateFailure) throw new ReleaseApprovalRequiredError(gateFailure);
    }

    const updated = this.repo.updateStatusInOrg(input.releaseId, input.organizationId, input.to);
    if (!updated) throw new ReleaseNotFoundError(input.releaseId);

    this.repo.appendTransition({
      id: this.createId(),
      releaseId: input.releaseId,
      fromStatus: existing.status,
      toStatus: input.to,
      actorId: input.actorId,
      reason: input.reason ?? null,
      createdAt: this.now(),
    });
    this.auditChain.append({
      userId: input.actorId,
      action: `release.${input.to}`,
      resource: 'release',
      details: JSON.stringify({
        releaseId: input.releaseId,
        from: existing.status,
        to: input.to,
        reason: input.reason,
      }),
      resourceKind: 'release',
      resourceId: input.releaseId,
    });
    return updated;
  }
}
