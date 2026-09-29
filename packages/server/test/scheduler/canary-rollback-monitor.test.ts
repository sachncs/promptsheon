import { describe, expect, it, vi } from 'vitest';
import { CanaryRollbackMonitor } from '../../src/scheduler/canary-rollback-monitor.js';

describe('CanaryRollbackMonitor', () => {
  it('assesses every discovered canary and prevents overlapping polls', async () => {
    const assess = vi.fn(() => ({ action: 'no_action' as const, reason: 'healthy', releaseId: 'release-a' }));
    const monitor = new CanaryRollbackMonitor(
      { listCanaryReleases: () => [{ releaseId: 'release-a', organizationId: 'org-a' }, { releaseId: 'release-b', organizationId: 'org-b' }] } as never,
      { assess } as never,
      { info: vi.fn(), error: vi.fn() },
    );

    await monitor.poll();

    expect(assess).toHaveBeenNthCalledWith(1, 'release-a', 'org-a', 'system:canary-monitor');
    expect(assess).toHaveBeenNthCalledWith(2, 'release-b', 'org-b', 'system:canary-monitor');
    expect(assess).toHaveBeenCalledTimes(2);
    monitor.stop();
  });
});
