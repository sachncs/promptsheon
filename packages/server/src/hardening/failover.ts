import type { AppConfig } from '@promptsheon/shared';
import { BudgetExceededError, type CostCheckResult } from './cost-caps.js';

export interface ModelCost {
  provider: string;
  modelId: string;
  costPerInputToken: number;
  costPerOutputToken: number;
}

export interface ModelDescriptor {
  provider: string;
  modelId: string;
}

/**
 * Static registry of well-known model costs (USD per token). Approximate
 * values from public pricing as of late 2025 — accuracy is not critical
 * here because cost-cap arithmetic uses `totalTokens * 0.00003`
 * elsewhere; this registry exists so failover can rank models by
 * cost-per-token when picking a cheaper alternative.
 *
 * New providers/models can be registered via `registerModel`.
 */
class ModelRegistry {
  private models = new Map<string, ModelCost>();

  constructor() {
    this.seedDefaults();
  }

  private seedDefaults(): void {
    const entries: ModelCost[] = [
      { provider: 'openai', modelId: 'gpt-4', costPerInputToken: 0.00003, costPerOutputToken: 0.00006 },
      { provider: 'openai', modelId: 'gpt-4o', costPerInputToken: 0.000005, costPerOutputToken: 0.000015 },
      { provider: 'openai', modelId: 'gpt-4o-mini', costPerInputToken: 0.00000015, costPerOutputToken: 0.0000006 },
      { provider: 'openai', modelId: 'gpt-3.5-turbo', costPerInputToken: 0.0000005, costPerOutputToken: 0.0000015 },
      { provider: 'anthropic', modelId: 'claude-3-opus', costPerInputToken: 0.000015, costPerOutputToken: 0.000075 },
      { provider: 'anthropic', modelId: 'claude-3-sonnet', costPerInputToken: 0.000003, costPerOutputToken: 0.000015 },
      { provider: 'anthropic', modelId: 'claude-3-haiku', costPerInputToken: 0.00000025, costPerOutputToken: 0.00000125 },
      { provider: 'bedrock', modelId: 'anthropic.claude-3-haiku', costPerInputToken: 0.00000025, costPerOutputToken: 0.00000125 },
      { provider: 'bedrock', modelId: 'anthropic.claude-3-sonnet', costPerInputToken: 0.000003, costPerOutputToken: 0.000015 },
    ];
    for (const e of entries) this.models.set(this.key(e.provider, e.modelId), e);
  }

  register(model: ModelCost): void {
    this.models.set(this.key(model.provider, model.modelId), model);
  }

  get(provider: string, modelId: string): ModelCost | undefined {
    return this.models.get(this.key(provider, modelId));
  }

  /**
   * List registered models for a provider sorted by total cost (cheapest first).
   */
  listForProvider(provider: string): ModelCost[] {
    return Array.from(this.models.values())
      .filter((m) => m.provider === provider)
      .sort((a, b) => this.totalCost(a) - this.totalCost(b));
  }

  totalCost(m: ModelCost): number {
    return m.costPerInputToken + m.costPerOutputToken;
  }

  private key(provider: string, modelId: string): string {
    return `${provider}::${modelId}`;
  }
}

const DEFAULT_REGISTRY = new ModelRegistry();

export function getModelRegistry(): ModelRegistry {
  return DEFAULT_REGISTRY;
}

/**
 * FailoverPolicy — on {@link BudgetExceededError} (per-invocation cap
 * exceeded), pick a cheaper model in the same provider and return a
 * new descriptor for the caller to use when retrying.
 *
 * The policy is intentionally stateless: callers pass the failing
 * model in, get a replacement out. No global state, no audit log.
 *
 * If no cheaper model exists in the same provider, falls back to
 * the cheapest model registered for any provider (typically
 * `gpt-3.5-turbo`).
 */
export class FailoverPolicy {
  constructor(
    private readonly config: AppConfig,
    private readonly registry: ModelRegistry = DEFAULT_REGISTRY,
  ) {}

  /**
   * Pick a failover model. Returns `null` when no cheaper model is
   * registered (caller should surface a 429 instead).
   */
  selectFailoverModel(current: ModelDescriptor, estimatedOutputTokens = 0): ModelDescriptor | null {
    const currentCost = this.registry.get(current.provider, current.modelId);
    if (!currentCost) return null;
    const currentTotal = this.registry.totalCost(currentCost);

    const providerCandidates = this.registry.listForProvider(current.provider).filter((m) => this.registry.totalCost(m) < currentTotal);
    if (providerCandidates.length > 0) {
      const cheapest = providerCandidates[0]!;
      return { provider: cheapest.provider, modelId: cheapest.modelId };
    }

    const all = Array.from(this.registry['models'].values() as Iterable<ModelCost>)
      .filter((m) => m.provider !== current.provider)
      .sort((a, b) => this.registry.totalCost(a) - this.registry.totalCost(b));
    if (all.length === 0) return null;
    void estimatedOutputTokens;
    const cheapest = all[0]!;
    return { provider: cheapest.provider, modelId: cheapest.modelId };
  }

  /**
   * Try to apply a failover after a {@link BudgetExceededError}. Returns
   * a new model descriptor, or `null` when no failover is available.
   */
  onBudgetExceeded(current: ModelDescriptor, error: BudgetExceededError): ModelDescriptor | null {
    if (error.scope !== 'per-invocation') return null;
    return this.selectFailoverModel(current);
  }
}

/**
 * Convenience helper for direct integration with `checkCostCap`: when
 * the cap result includes a `failoverModel`, return it as a parsed
 * {@link ModelDescriptor}. Otherwise return `null`.
 */
export function failoverDescriptorFromCostCheck(result: CostCheckResult): ModelDescriptor | null {
  if (!result.failoverModel) return null;
  const [provider, modelId] = result.failoverModel.split('/');
  if (!provider || !modelId) return null;
  return { provider, modelId };
}