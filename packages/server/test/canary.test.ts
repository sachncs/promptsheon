import { describe, it, expect } from 'vitest';
import { selectByCanary } from '../src/routes/release.js';

describe('selectByCanary', () => {
  it('returns null on empty pool', () => {
    expect(selectByCanary([])).toBeNull();
  });

  it('returns the only release when pool has one', () => {
    expect(selectByCanary([{ id: 'r1', canaryPercent: 50 }])).toBe('r1');
  });

  it('returns the only release even at 0% canary', () => {
    expect(selectByCanary([{ id: 'r1', canaryPercent: 0 }])).toBe('r1');
  });

  it('respects weighted distribution: 100% always picks r1', () => {
    const rng = () => 0.5;
    for (let i = 0; i < 50; i++) {
      expect(selectByCanary([
        { id: 'r1', canaryPercent: 100 },
        { id: 'r2', canaryPercent: 0 },
      ], rng)).toBe('r1');
    }
  });

  it('respects weighted distribution: 30/70 split', () => {
    let r1Count = 0;
    let r2Count = 0;
    for (let i = 0; i < 1000; i++) {
      const id = selectByCanary([
        { id: 'r1', canaryPercent: 30 },
        { id: 'r2', canaryPercent: 70 },
      ]);
      if (id === 'r1') r1Count++;
      if (id === 'r2') r2Count++;
    }
    expect(r1Count).toBeGreaterThan(200);
    expect(r1Count).toBeLessThan(400);
    expect(r2Count).toBeGreaterThan(600);
    expect(r2Count).toBeLessThan(800);
  });

  it('falls back to last release if rng exceeds total', () => {
    const rng = () => 0.99999;
    expect(selectByCanary([
      { id: 'r1', canaryPercent: 50 },
      { id: 'r2', canaryPercent: 50 },
    ], rng)).toBe('r2');
  });

  it('returns first release if all canaryPercents are 0', () => {
    const rng = () => 0.5;
    expect(selectByCanary([
      { id: 'r1', canaryPercent: 0 },
      { id: 'r2', canaryPercent: 0 },
    ], rng)).toBe('r1');
  });
});