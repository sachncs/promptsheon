import { ModelError } from '@strands-agents/sdk';

export interface CostLimitConfig {
  /** Per-call USD cap. Hard reject (throw) if estimated cost > this. */
  perInvocationUsd: number;
  /** Org-level daily cap. */
  orgDailyUsd: number;
  /** Capability-level daily cap. */
  capabilityDailyUsd: number;
  /** Where to store the in-memory cost ledger. */
  ledger?: CostLedger;
}

export interface CostLedger {
  record(orgId: string, capabilityId: string, costUsd: number): void;
  todayUsed(orgId: string, capabilityId?: string): number;
}

/**
 * In-memory cost ledger. Production v2 would back this with
 * Postgres/SQLite.
 */
export class InMemoryCostLedger implements CostLedger {
  private entries: Array<{ orgId: string; capabilityId: string; costUsd: number; ts: number }> = [];
  record(orgId: string, capabilityId: string, costUsd: number): void {
    this.entries.push({ orgId, capabilityId, costUsd, ts: Date.now() });
  }
  todayUsed(orgId: string, capabilityId?: string): number {
    const cutoff = Date.now() - 24 * 60 * 60 * 1000;
    return this.entries
      .filter((e) => e.orgId === orgId && e.ts > cutoff && (capabilityId === undefined || e.capabilityId === capabilityId))
      .reduce((sum, e) => sum + e.costUsd, 0);
  }
}

export class BudgetExceededError extends ModelError {
  constructor(public readonly budget: number, public readonly attempted: number, public readonly scope: 'per-invocation' | 'org-daily' | 'capability-daily') {
    super(`budget exceeded (${scope}): attempted ${attempted}, limit ${budget}`);
    this.name = 'BudgetExceededError';
  }
}

export interface CostCheckInput {
  orgId: string;
  capabilityId: string;
  estimatedCostUsd: number;
  config: CostLimitConfig;
}

export interface CostCheckResult {
  allowed: boolean;
  reason?: string;
  failoverModel?: string;
}

/**
 * Check whether a proposed LLM call is allowed under the configured caps.
 * Returns allowed=false with a reason when blocked.
 *
 * When a non-blocking cap is hit (e.g. org daily near limit), the
 * caller can pass `allowFailover: true` to get a cheaper-model
 * recommendation.
 */
export function checkCostCap(input: CostCheckInput, opts: { allowFailover?: boolean } = {}): CostCheckResult {
  const { orgId, capabilityId, estimatedCostUsd, config } = input;
  if (estimatedCostUsd > config.perInvocationUsd) {
    return { allowed: false, reason: `per-invocation $${config.perInvocationUsd} exceeded` };
  }
  const capToday = config.ledger ?? new InMemoryCostLedger();
  const orgToday = capToday.todayUsed(orgId);
  if (orgToday + estimatedCostUsd > config.orgDailyUsd) {
    if (opts.allowFailover) {
      return { allowed: true, reason: 'org-daily cap exceeded but failover accepted', failoverModel: 'gpt-3.5-turbo' };
    }
    return { allowed: false, reason: `org-daily $${config.orgDailyUsd} exceeded` };
  }
  const capToday_ = capToday.todayUsed(orgId, capabilityId);
  if (capToday_ + estimatedCostUsd > config.capabilityDailyUsd) {
    if (opts.allowFailover) {
      return { allowed: true, reason: 'capability-daily cap exceeded but failover accepted', failoverModel: 'gpt-3.5-turbo' };
    }
    return { allowed: false, reason: `capability-daily $${config.capabilityDailyUsd} exceeded` };
  }
  return { allowed: true };
}

/**
 * Record cost after invocation. Use the same CostLimitConfig to
 * share the ledger across checks.
 */
export function recordCost(orgId: string, capabilityId: string, costUsd: number, config: CostLimitConfig): void {
  const ledger = config.ledger ?? new InMemoryCostLedger();
  ledger.record(orgId, capabilityId, costUsd);
}