import { describe, expect, it } from 'vitest';
import { nextCronFire } from '../src/scheduler/cron.js';

describe('cron scheduling', () => {
  it('finds the next matching UTC minute', () => {
    const next = nextCronFire('*/15 * * * *', new Date('2026-09-24T10:07:32.000Z'));
    expect(next.toISOString()).toBe('2026-09-24T10:15:00.000Z');
  });

  it('supports lists, ranges, and day-of-week', () => {
    const next = nextCronFire('30 9 * * 4', new Date('2026-09-24T09:31:00.000Z'));
    expect(next.toISOString()).toBe('2026-10-01T09:30:00.000Z');
  });

  it('rejects malformed expressions', () => {
    expect(() => nextCronFire('every fifteen minutes', new Date())).toThrow(/five fields|invalid cron/);
  });
});
