import { afterEach, describe, expect, it, vi } from 'vitest';
import { Scheduler } from '../src/scheduler/scheduler.js';

describe('Scheduler lifecycle', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it('stops polling after stop is called', async () => {
    vi.useFakeTimers();
    const findDueSchedules = vi.fn().mockResolvedValue([]);
    const scheduler = new Scheduler(
      { findDueSchedules } as never,
      { broadcast: vi.fn() } as never,
    );

    scheduler.start(1000);
    await vi.advanceTimersByTimeAsync(1000);
    const callsBeforeStop = findDueSchedules.mock.calls.length;
    expect(callsBeforeStop).toBeGreaterThanOrEqual(2);

    scheduler.stop();
    await vi.advanceTimersByTimeAsync(5000);
    expect(findDueSchedules).toHaveBeenCalledTimes(callsBeforeStop);
  });

  it('does not create duplicate polling intervals when started twice', async () => {
    vi.useFakeTimers();
    const findDueSchedules = vi.fn().mockResolvedValue([]);
    const scheduler = new Scheduler(
      { findDueSchedules } as never,
      { broadcast: vi.fn() } as never,
    );

    scheduler.start(1000);
    scheduler.start(1000);
    await vi.advanceTimersByTimeAsync(1000);

    expect(findDueSchedules).toHaveBeenCalledTimes(2);
    scheduler.stop();
  });

  it('advances a successful schedule instead of leaving it due immediately', async () => {
    const schedule = {
      id: 'schedule-1',
      workspaceId: 'workspace-1',
      releaseId: 'release-1',
      kind: 'test',
      cron: '* * * * *',
      webhookPath: '',
      nextFireAt: new Date(0).toISOString(),
      lastFireAt: null,
      firedCount: 0,
      enabled: true,
      createdAt: new Date(0).toISOString(),
      createdBy: 'test',
    };
    const handler = vi.fn().mockResolvedValue(undefined);
    const advance = vi.fn().mockResolvedValue(schedule);
    const scheduler = new Scheduler(
      {
        findDueSchedules: vi.fn().mockResolvedValue([schedule]),
        advance,
      } as never,
      { broadcast: vi.fn() } as never,
    );
    scheduler.registerHandler('test', handler);

    await scheduler.poll();

    expect(handler).toHaveBeenCalledWith(schedule);
    expect(advance).toHaveBeenCalledWith(schedule.id, expect.any(Date));
  });
});
