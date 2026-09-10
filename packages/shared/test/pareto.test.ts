import { describe, it, expect } from 'vitest';
import { paretoFrontier, pickCheapestFrontier } from '@promptsheon/shared';

describe('pareto-frontier (multi-axis meta learner)', () => {
  it('a single point is on the frontier', () => {
    const f = paretoFrontier([{ label: 'A', passRate: 0.9, resistance: 0.8, costMicros: 100 }]);
    expect(f).toHaveLength(1);
    expect(f[0]?.isFrontier).toBe(true);
  });

  it('a clearly inferior point is not on the frontier', () => {
    const f = paretoFrontier([
      { label: 'A', passRate: 0.9, resistance: 0.8, costMicros: 100 },
      { label: 'B', passRate: 0.5, resistance: 0.3, costMicros: 80 },
    ]);
    expect(f.find((p) => p.label === 'A')?.isFrontier).toBe(true);
    expect(f.find((p) => p.label === 'B')?.isFrontier).toBe(false);
    expect(f.find((p) => p.label === 'B')?.dominatedBy).toEqual(['A']);
  });

  it('two trade-offs end up both on the frontier', () => {
    const f = paretoFrontier([
      { label: 'high-pass', passRate: 0.95, resistance: 0.4, costMicros: 100 },
      { label: 'high-resistance', passRate: 0.5, resistance: 0.95, costMicros: 100 },
    ]);
    expect(f.every((p) => p.isFrontier)).toBe(true);
  });

  it('a dominated point carries a retune hint pointing at the dominator', () => {
    const f = paretoFrontier([
      { label: 'A', passRate: 0.95, resistance: 0.9, costMicros: 100 },
      { label: 'B', passRate: 0.6, resistance: 0.7, costMicros: 80 },
    ]);
    const b = f.find((p) => p.label === 'B');
    expect(b?.isFrontier).toBe(false);
    expect(b?.retuneHint).toContain('dominated by "A"');
  });

  it('pickCheapestFrontier picks the cheapest frontier point at threshold', () => {
    const f = paretoFrontier([
      { label: 'A', passRate: 0.95, resistance: 0.9, costMicros: 200 },
      { label: 'B', passRate: 0.93, resistance: 0.85, costMicros: 100 },
      { label: 'C', passRate: 0.91, resistance: 0.7, costMicros: 50 },
    ]);
    const pick = pickCheapestFrontier(f, 0.9);
    expect(pick?.label).toBe('C');
  });

  it('pickCheapestFrontier returns null when no frontier point meets threshold', () => {
    const f = paretoFrontier([
      { label: 'A', passRate: 0.5, resistance: 0.5, costMicros: 100 },
    ]);
    const pick = pickCheapestFrontier(f, 0.9);
    expect(pick).toBeNull();
  });

  it('non-dominated boundary cases stay on the frontier', () => {
    // A, B, C are non-dominated (A is best pass, C is best resistance,
    // B is a trade-off in between).
    const f = paretoFrontier([
      { label: 'A', passRate: 0.95, resistance: 0.6, costMicros: 100 },
      { label: 'B', passRate: 0.85, resistance: 0.8, costMicros: 100 },
      { label: 'C', passRate: 0.7, resistance: 0.95, costMicros: 100 },
    ]);
    const frontierLabels = f.filter((p) => p.isFrontier).map((p) => p.label).sort();
    expect(frontierLabels).toEqual(['A', 'B', 'C']);
  });
});
