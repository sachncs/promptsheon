import { describe, expect, it, vi } from 'vitest';
import { CanaryRollbackService } from '../../src/application/canary-rollback-service.js';
import type { Release } from '@promptsheon/shared';

const canary: Release = {
  id: 'release-canary', capabilityId: 'cap-1', capabilityVersion: 2, capabilityVersionId: null,
  manifest: '{}', environment: 'prod', status: 'canary', approvedBy: '', replacesReleaseId: null,
  createdAt: '2026-01-02T00:00:00.000Z', createdBy: 'operator', activatedAt: null, canaryPercent: 10,
};
const stable: Release = { ...canary, id: 'release-stable', capabilityVersion: 1, status: 'active', canaryPercent: 0 };

describe('CanaryRollbackService', () => {
  it('atomically rolls back a canary after a failed evaluation', () => {
    const rollback = vi.fn(() => ({ rolledBack: { ...canary, status: 'rolled_back' }, reactivated: stable }));
    const audit = { append: vi.fn() };
    const service = new CanaryRollbackService(
      { findByIdInOrg: vi.fn(() => canary), findActivePeerInOrg: vi.fn(() => stable), rollbackAtomicallyInOrg: rollback } as never,
      { findRunsByReleaseIdInOrg: vi.fn(() => [{ id: 'eval-1', status: 'failed', score: 0.2 }]) } as never,
      audit as never,
    );

    const result = service.assess(canary.id, 'org-1', 'system');

    expect(result).toMatchObject({ action: 'rolled_back', targetReleaseId: stable.id, evaluationId: 'eval-1' });
    expect(rollback).toHaveBeenCalledWith(canary.id, stable.id, 'org-1');
    expect(audit.append).toHaveBeenCalledOnce();
  });

  it('leaves a healthy canary untouched', () => {
    const rollback = vi.fn();
    const service = new CanaryRollbackService(
      { findByIdInOrg: vi.fn(() => canary), findActivePeerInOrg: vi.fn(), rollbackAtomicallyInOrg: rollback } as never,
      { findRunsByReleaseIdInOrg: vi.fn(() => [{ id: 'eval-2', status: 'passed', score: 0.95 }]) } as never,
      { append: vi.fn() } as never,
    );

    expect(service.assess(canary.id, 'org-1', 'system')).toMatchObject({ action: 'no_action', reason: 'evaluation is healthy' });
    expect(rollback).not.toHaveBeenCalled();
  });
});
