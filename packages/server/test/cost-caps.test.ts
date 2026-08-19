import { describe, it, expect } from 'vitest';
import {
  InMemoryCostLedger,
  checkCostCap,
  recordCost,
  BudgetExceededError,
} from '../src/hardening/cost-caps.js';

function makeConfig(overrides: Record<string, unknown> = {}) {
  return {
    perInvocationUsd: 0.10,
    orgDailyUsd: 1.00,
    capabilityDailyUsd: 0.50,
    ledger: new InMemoryCostLedger(),
    ...overrides,
  };
}

describe('checkCostCap', () => {
  it('allows a call within all caps', () => {
    const result = checkCostCap({ orgId: 'o1', capabilityId: 'c1', estimatedCostUsd: 0.01, config: makeConfig() });
    expect(result.allowed).toBe(true);
  });

  it('blocks per-invocation cap when estimate exceeds', () => {
    const result = checkCostCap({ orgId: 'o1', capabilityId: 'c1', estimatedCostUsd: 0.50, config: makeConfig() });
    expect(result.allowed).toBe(false);
    expect(result.reason).toContain('per-invocation');
  });

  it('blocks org-daily cap when total would exceed', () => {
    const cfg = makeConfig();
    recordCost('o1', 'c1', 0.95, cfg);
    const result = checkCostCap({ orgId: 'o1', capabilityId: 'c1', estimatedCostUsd: 0.10, config: cfg });
    expect(result.allowed).toBe(false);
  });

  it('blocks capability-daily cap when total would exceed', () => {
    const cfg = makeConfig();
    recordCost('o1', 'c1', 0.45, cfg);
    const result = checkCostCap({ orgId: 'o1', capabilityId: 'c1', estimatedCostUsd: 0.10, config: cfg });
    expect(result.allowed).toBe(false);
  });

  it('returns failover model when org cap hit and failover allowed', () => {
    const cfg = makeConfig();
    recordCost('o1', 'c1', 0.95, cfg);
    const result = checkCostCap({ orgId: 'o1', capabilityId: 'c1', estimatedCostUsd: 0.10, config: cfg }, { allowFailover: true });
    expect(result.allowed).toBe(true);
    expect(result.failoverModel).toBe('gpt-3.5-turbo');
  });
});

describe('recordCost + ledger', () => {
  it('accumulates per org + capability', () => {
    const cfg = makeConfig();
    recordCost('o1', 'c1', 0.10, cfg);
    recordCost('o1', 'c1', 0.20, cfg);
    recordCost('o1', 'c2', 0.30, cfg);
    expect(cfg.ledger!.todayUsed('o1', 'c1')).toBeCloseTo(0.30, 5);
    expect(cfg.ledger!.todayUsed('o1', 'c2')).toBeCloseTo(0.30, 5);
    expect(cfg.ledger!.todayUsed('o1')).toBeCloseTo(0.60, 5);
  });
});

describe('BudgetExceededError', () => {
  it('carries budget, attempted, scope', () => {
    const e = new BudgetExceededError(1.0, 2.0, 'org-daily');
    expect(e.budget).toBe(1.0);
    expect(e.attempted).toBe(2.0);
    expect(e.scope).toBe('org-daily');
    expect(e.name).toBe('BudgetExceededError');
  });
});