import { describe, expect, it, vi } from 'vitest';
import { EvalSuiteService, type EvalSuiteStore, type GraderFactory } from '../../src/application/eval-suite-service.js';
import type { EvalSuite, EvalSuiteVersion } from '@promptsheon/shared';

const suite: EvalSuite = {
  id: 'suite-1',
  capabilityId: 'capability-1',
  repositoryId: 'repo-1',
  name: 'Quality suite',
  description: null,
  currentVersion: 1,
  passThreshold: 0.9,
  borderlineBand: 0.1,
  createdBy: 'user-1',
  createdAt: '2026-01-01T00:00:00.000Z',
  updatedAt: '2026-01-01T00:00:00.000Z',
};

const version: EvalSuiteVersion = {
  id: 'version-1',
  suiteId: suite.id,
  version: 1,
  graderConfig: [],
  passThreshold: suite.passThreshold,
  borderlineBand: suite.borderlineBand,
  k: 1,
  n: 1,
  notes: null,
  createdBy: 'user-1',
  createdAt: suite.createdAt,
};

function dependencies() {
  const store: EvalSuiteStore = {
    findById: vi.fn(() => suite),
    findByIdInOrg: vi.fn((id, organizationId) => (id === suite.id && organizationId === 'org-a' ? suite : null)),
    findVersion: vi.fn(() => version),
    findVersionInOrg: vi.fn((id, currentVersion, organizationId) =>
      id === suite.id && currentVersion === version.version && organizationId === 'org-a' ? version : null,
    ),
    findVersionById: vi.fn(() => version),
    findVersionByIdInOrg: vi.fn((id, organizationId) => id === version.id && organizationId === 'org-a' ? version : null),
    list: vi.fn(() => [suite]),
    listForRepositoryInOrg: vi.fn((repositoryId, organizationId) =>
      repositoryId === suite.repositoryId && organizationId === 'org-a' ? [suite] : [],
    ),
  };
  const grader: GraderFactory = {
    create: vi.fn(() => ({
      run: vi.fn((trial) => ({
        results: [{ caseId: trial.caseId }],
        weightedScore: trial.caseId === 'borderline' ? 0.85 : 1,
        passed: trial.caseId !== 'borderline',
      })),
    })),
  };
  const reviews = { enqueue: vi.fn() };
  return { store, grader, reviews };
}

describe('EvalSuiteService', () => {
  it('does not run a suite across organization boundaries', () => {
    const deps = dependencies();
    const service = new EvalSuiteService(deps.store, deps.grader, deps.reviews);

    expect(service.run(suite.id, 'org-b', { trials: [] })).toEqual({ kind: 'suite-not-found' });
    expect(deps.grader.create).not.toHaveBeenCalled();
  });

  it('selects the tenant-scoped version, calculates scores, and queues borderline trials', () => {
    const deps = dependencies();
    const service = new EvalSuiteService(deps.store, deps.grader, deps.reviews);

    const result = service.run('suite-1', 'org-a', {
      n: 2,
      k: 1,
      trials: [
        { caseId: 'passing', output: 'ok' },
        { caseId: 'borderline', output: 'needs review' },
      ],
    });

    expect(result.kind).toBe('success');
    if (result.kind !== 'success') return;
    expect(result.value.rawScore).toBeCloseTo(0.925);
    expect(result.value.passAtK).toBe(0.5);
    expect(result.value.borderlineCount).toBe(1);
    expect(deps.reviews.enqueue).toHaveBeenCalledWith('borderline', suite.id, null);
    expect(deps.store.findByIdInOrg).toHaveBeenCalledWith(suite.id, 'org-a');
    expect(deps.store.findVersionInOrg).toHaveBeenCalledWith(suite.id, suite.currentVersion, 'org-a');
  });

  it('returns a version error before invoking the grader', () => {
    const deps = dependencies();
    vi.mocked(deps.store.findVersionInOrg).mockReturnValue(null);
    const service = new EvalSuiteService(deps.store, deps.grader, deps.reviews);

    expect(service.run(suite.id, 'org-a', {})).toEqual({ kind: 'version-not-found' });
    expect(deps.grader.create).not.toHaveBeenCalled();
  });

  it('evaluates every repository suite through its current version', () => {
    const deps = dependencies();
    const service = new EvalSuiteService(deps.store, deps.grader, deps.reviews);

    const result = service.gate('repo-1', 'org-a', [{ caseId: 'passing', output: 'ok' }]);

    expect(result).toEqual({
      ok: true,
      score: 1,
      regressions: [],
      suites: [{ suiteId: suite.id, suiteName: suite.name, ok: true, rawScore: 1, threshold: suite.passThreshold }],
    });
    expect(deps.store.listForRepositoryInOrg).toHaveBeenCalledWith('repo-1', 'org-a');
  });
});
