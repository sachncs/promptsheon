import { describe, expect, it, vi } from 'vitest';
import {
  InvalidReleaseTransitionError,
  ReleaseApprovalRequiredError,
  ReleaseService,
  type AuditWriter,
  type ManifestApprovalStore,
  type ReleaseStore,
} from '../../src/application/release-service.js';
import type { Release } from '@promptsheon/shared';

const release: Release = {
  id: 'release-1',
  capabilityId: 'capability-1',
  capabilityVersion: 1,
  capabilityVersionId: null,
  manifest: '{}',
  environment: 'prod',
  status: 'review',
  approvedBy: '',
  replacesReleaseId: null,
  createdAt: '2026-01-01T00:00:00.000Z',
  createdBy: 'creator',
  activatedAt: null,
  canaryPercent: 0,
};

function makeService(approvers: string[] = ['reviewer-a', 'reviewer-b']) {
  const store: ReleaseStore = {
    findByIdInOrg: vi.fn(() => release),
    updateStatusInOrg: vi.fn((_id, _org, status) => ({ ...release, status })),
    appendTransition: vi.fn(),
  };
  const approvals: ManifestApprovalStore = {
    computeManifestHash: vi.fn(() => 'manifest-hash'),
    findApprovals: vi.fn(() => approvers.map((userId) => ({ userId }))),
  };
  const audit: AuditWriter = { append: vi.fn() };
  const service = new ReleaseService(store, approvals, audit, {
    createId: () => 'transition-1',
    now: () => '2026-01-01T00:01:00.000Z',
  });
  return { service, store, approvals, audit };
}

describe('ReleaseService', () => {
  it('transitions a release and records history and audit data', () => {
    const { service, store, audit } = makeService();

    const result = service.transition({
      releaseId: release.id,
      organizationId: 'org-1',
      actorId: 'operator',
      to: 'approved',
      reason: 'verified',
    });

    expect(result.status).toBe('approved');
    expect(store.updateStatusInOrg).toHaveBeenCalledWith(release.id, 'org-1', 'approved');
    expect(store.appendTransition).toHaveBeenCalledWith({
      id: 'transition-1',
      releaseId: release.id,
      fromStatus: 'review',
      toStatus: 'approved',
      actorId: 'operator',
      reason: 'verified',
      createdAt: '2026-01-01T00:01:00.000Z',
    });
    expect(audit.append).toHaveBeenCalledOnce();
  });

  it('rejects invalid transitions before mutating persistence', () => {
    const { service, store } = makeService();

    expect(() => service.transition({
      releaseId: release.id,
      organizationId: 'org-1',
      actorId: 'operator',
      to: 'active',
    })).toThrow(InvalidReleaseTransitionError);
    expect(store.updateStatusInOrg).not.toHaveBeenCalled();
  });

  it('rejects activation when the approval gate is not satisfied', () => {
    const { service, store } = makeService(['reviewer-a']);

    expect(() => service.transition({
      releaseId: release.id,
      organizationId: 'org-1',
      actorId: 'operator',
      to: 'approved',
    })).toThrow(ReleaseApprovalRequiredError);
    expect(store.updateStatusInOrg).not.toHaveBeenCalled();
  });
});
