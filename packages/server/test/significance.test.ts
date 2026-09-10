import { describe, it, expect } from 'vitest';
import {
  bayesianCompare,
  buildSignificanceReport,
  normalCdf,
  twoProportionZTest,
  type VariantStats,
} from '../src/analysis/significance.js';

describe('significance: normalCdf', () => {
  it('returns 0.5 at z=0', () => {
    expect(normalCdf(0)).toBeCloseTo(0.5, 7);
  });
  it('returns ~0.975 at z=1.96', () => {
    expect(normalCdf(1.96)).toBeCloseTo(0.975, 3);
  });
  it('returns ~0.025 at z=-1.96', () => {
    expect(normalCdf(-1.96)).toBeCloseTo(0.025, 3);
  });
  it('is monotonically increasing', () => {
    const xs = [-3, -2, -1, 0, 1, 2, 3];
    let prev = -1;
    for (const x of xs) {
      const v = normalCdf(x);
      expect(v).toBeGreaterThan(prev);
      prev = v;
    }
  });
  it('clamps to ~1 for large positive z', () => {
    expect(normalCdf(8)).toBeGreaterThan(0.99999999);
  });
  it('clamps to ~0 for large negative z', () => {
    expect(normalCdf(-8)).toBeLessThan(1e-8);
  });
});

describe('significance: twoProportionZTest', () => {
  it('null when either sample is empty', () => {
    expect(twoProportionZTest({ passes: 0, total: 0 }, { passes: 10, total: 100 })).toBeNull();
    expect(twoProportionZTest({ passes: 10, total: 100 }, { passes: 0, total: 0 })).toBeNull();
  });
  it('p≈1 for identical rates', () => {
    const r = twoProportionZTest({ passes: 50, total: 100 }, { passes: 50, total: 100 });
    expect(r).not.toBeNull();
    expect(r!.pValue).toBeGreaterThan(0.5);
  });
  it('p<0.05 for a 50/50 vs 70/70 split', () => {
    const r = twoProportionZTest({ passes: 50, total: 100 }, { passes: 70, total: 100 });
    expect(r).not.toBeNull();
    expect(r!.pValue).toBeLessThan(0.05);
    expect(r!.significant ?? false).toBe(false);
  });
  it('meanDiff is positive when A>B', () => {
    const r = twoProportionZTest({ passes: 60, total: 100 }, { passes: 30, total: 100 });
    expect(r!.meanDiff).toBeCloseTo(0.3, 5);
  });
  it('Wald CI95 covers the true zero when rates are equal', () => {
    const r = twoProportionZTest({ passes: 50, total: 100 }, { passes: 50, total: 100 });
    expect(r!.diffCi95[0]).toBeLessThanOrEqual(0);
    expect(r!.diffCi95[1]).toBeGreaterThanOrEqual(0);
  });
});

describe('significance: bayesianCompare', () => {
  it('P(A>B) ~0.5 when rates are equal', () => {
    const b = bayesianCompare({ passes: 50, fails: 50 }, { passes: 50, fails: 50 }, { samples: 5000 });
    expect(b.probABeatsB).toBeGreaterThan(0.4);
    expect(b.probABeatsB).toBeLessThan(0.6);
  });
  it('P(A>B) is high when A dominates', () => {
    const b = bayesianCompare({ passes: 90, fails: 10 }, { passes: 30, fails: 70 }, { samples: 5000 });
    expect(b.probABeatsB).toBeGreaterThan(0.99);
  });
  it('credible interval brackets the true rate', () => {
    const b = bayesianCompare({ passes: 50, fails: 50 }, { passes: 50, fails: 50 }, { samples: 5000 });
    expect(b.credibleIntervalA[0]).toBeLessThan(0.5);
    expect(b.credibleIntervalA[1]).toBeGreaterThan(0.5);
    expect(b.credibleIntervalA[0]).toBeGreaterThan(0.2);
    expect(b.credibleIntervalA[1]).toBeLessThan(0.8);
  });
});

describe('significance: buildSignificanceReport', () => {
  const observed: VariantStats[] = [
    { label: 'control', cases: 100, passes: 50, fails: 50, passRate: 0.5 },
    { label: 'treatment', cases: 100, passes: 70, fails: 30, passRate: 0.7 },
  ];
  it('reports one pairwise test for two variants', () => {
    const r = buildSignificanceReport(observed, { bayesSamples: 1000 });
    expect(r).not.toBeNull();
    expect(r!.pairwise).toHaveLength(1);
    expect(r!.bayesian).toHaveLength(1);
  });
  it('flags the higher-rate variant as the winner', () => {
    const r = buildSignificanceReport(observed, { bayesSamples: 1000 })!;
    expect(r.winner).toBe('treatment');
    expect(r.anySignificant).toBe(true);
    expect(r.ranking[0]).toBe('treatment');
  });
  it('returns null when no variant has observations', () => {
    expect(buildSignificanceReport([
      { label: 'a', cases: 0, passes: 0, fails: 0, passRate: 0 },
      { label: 'b', cases: 0, passes: 0, fails: 0, passRate: 0 },
    ])).toBeNull();
  });
  it('marks close rates as inconclusive (no winner)', () => {
    const r = buildSignificanceReport([
      { label: 'a', cases: 100, passes: 51, fails: 49, passRate: 0.51 },
      { label: 'b', cases: 100, passes: 49, fails: 51, passRate: 0.49 },
    ], { bayesSamples: 1000 });
    expect(r).not.toBeNull();
    expect(r!.anySignificant).toBe(false);
    expect(r!.winner).toBeNull();
  });
  it('handles three variants pairwise', () => {
    const r = buildSignificanceReport([
      { label: 'a', cases: 100, passes: 50, fails: 50, passRate: 0.5 },
      { label: 'b', cases: 100, passes: 70, fails: 30, passRate: 0.7 },
      { label: 'c', cases: 100, passes: 30, fails: 70, passRate: 0.3 },
    ], { bayesSamples: 500 });
    expect(r!.pairwise).toHaveLength(3);
    expect(r!.ranking).toEqual(['b', 'a', 'c']);
    expect(r!.winner).toBe('b');
  });
});