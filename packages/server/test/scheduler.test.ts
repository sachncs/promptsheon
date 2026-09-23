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
});
