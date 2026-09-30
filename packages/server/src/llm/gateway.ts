import { createHash } from 'node:crypto';
import type { LlmRouter } from '../llm/router.js';
import { CircuitBreaker } from '../application/execution-resilience.js';

/**
 * ResponseCache — content-hash-keyed response cache for the
 * promptsheon gateway. Same prompt + same model + same temperature
 * + same provider → same hash → cache hit.
 *
 * Storage is a Map (process-local, LRU-bounded) plus an optional
 * SQLite-backed secondary tier so the cache survives restarts.
 * Cache keys never include PII fields; the caller is responsible
 * for redacting inputs before calling .get / .set.
 */
export interface CacheEntry {
  hash: string;
  prompt: string;
  model: string;
  temperature: number;
  provider: string;
  baseUrl?: string;
  content: string;
  promptTokens: number;
  completionTokens: number;
  costUsd: number;
  createdAt: string;
}

export interface CacheLookup {
  prompt: string;
  model: string;
  temperature: number;
  provider: string;
  /** Optional endpoint identity for custom OpenAI/Anthropic-compatible providers. */
  baseUrl?: string;
  /** Tenant or actor namespace; responses must never cross this boundary. */
  scopeId?: string;
}

export interface ResponseCacheStore {
  get(hash: string): CacheEntry | null;
  set(entry: CacheEntry): void;
  delete(hash: string): void;
  clear(): void;
  size(): number;
  trim(maxEntries: number): void;
}

function cacheKey(input: CacheLookup): string {
  // Deterministic: same prompt + model + temperature → same hash,
  // regardless of which provider the caller first tried. The
  // fallback chain is an implementation detail of the gateway;
  // the cache is keyed on what the caller asked for so a hit
  // returns the cached provider transparently.
  const payload = JSON.stringify({
    model: input.model,
    prompt: input.prompt,
    temperature: input.temperature,
    provider: input.provider,
    baseUrl: input.baseUrl ?? null,
    scopeId: input.scopeId ?? null,
  });
  return createHash('sha256').update(payload).digest('hex');
}

export class ResponseCache {
  private readonly store = new Map<string, CacheEntry>();
  private readonly maxEntries: number;

  constructor(
    maxEntries = 1024,
    private readonly persistentStore?: ResponseCacheStore,
    private readonly maxAgeMs = Number.POSITIVE_INFINITY,
  ) {
    if (!Number.isInteger(maxEntries) || maxEntries < 1) throw new Error('maxEntries must be positive');
    if (!(maxAgeMs > 0)) throw new Error('maxAgeMs must be positive');
    this.maxEntries = maxEntries;
  }

  /**
   * Lookup a cached response. Returns `null` on miss. Updates LRU
   * order on hit.
   */
  get(input: CacheLookup): CacheEntry | null {
    const hash = cacheKey(input);
    const entry = this.store.get(hash);
    if (entry && this.isExpired(entry)) {
      this.store.delete(hash);
      this.persistentStore?.delete(hash);
      return null;
    }
    if (!entry) {
      const persisted = this.persistentStore?.get(hash) ?? null;
      if (!persisted) return null;
      if (this.isExpired(persisted)) {
        this.persistentStore?.delete(hash);
        return null;
      }
      this.store.set(hash, persisted);
      return persisted;
    }
    // Refresh LRU order.
    this.store.delete(hash);
    this.store.set(hash, entry);
    return entry;
  }

  /**
   * Insert an entry. If the cache is at capacity, evict the
   * least-recently-used entry (Map iteration order is insertion
   * order; we delete + re-insert on hit; oldest unreferenced
   * entry is first on insertion order).
   */
  set(input: CacheLookup & Omit<CacheEntry, 'hash' | 'createdAt'>, keyInput: CacheLookup = input): CacheEntry {
    const hash = cacheKey(keyInput);
    const entry: CacheEntry = { ...input, hash, createdAt: new Date().toISOString() };
    if (this.store.has(hash)) this.store.delete(hash);
    this.store.set(hash, entry);
    while (this.store.size > this.maxEntries) {
      const oldest = this.store.keys().next().value;
      if (oldest === undefined) break;
      this.store.delete(oldest);
    }
    this.persistentStore?.set(entry);
    this.persistentStore?.trim(this.maxEntries);
    return entry;
  }

  size(): number {
    return this.persistentStore?.size() ?? this.store.size;
  }

  clear(): void {
    this.store.clear();
    this.persistentStore?.clear();
  }

  private isExpired(entry: CacheEntry): boolean {
    return Date.now() - Date.parse(entry.createdAt) >= this.maxAgeMs;
  }
}

/** Exposed for tests so callers can verify the cache key directly. */
export { cacheKey };

/**
 * FallbackChain — sequential provider fallback. Tries providers
 * in declared order; on any non-recoverable failure, retries
 * with the next provider. Used by the gateway so a single
 * upstream outage doesn't take the platform offline.
 */
export class FallbackChain {
  private readonly providers: string[];
  constructor(providers: string[]) {
    if (providers.length === 0) throw new Error('FallbackChain requires at least one provider');
    this.providers = [...providers];
  }

  order(): string[] {
    return [...this.providers];
  }
}

/**
 * Gateway — public surface that the LLM router delegates to
 * before contacting a provider. Order:
 *   1. compute request hash
 *   2. cache lookup (return on hit)
 *   3. forward to LlmRouter
 *   4. cache the response
 *   5. fall through the FallbackChain on provider failure
 */
export interface GatewayRequest extends CacheLookup {
  stream?: boolean;
  signal?: AbortSignal;
  baseUrl?: string;
  apiKey?: string;
}

export interface GatewayResponse {
  content: string;
  provider: string;
  model: string;
  promptTokens: number;
  completionTokens: number;
  costUsd: number;
  cacheHit: boolean;
  latencyMs: number;
}

export interface RateLimitState {
  tokensRemaining: number;
  resetAt: number;
}

/** Aggregate rate-limit counters safe to expose to operators. */
export interface RateLimiterMetrics {
  activeBuckets: number;
  maxBuckets: number;
  totalRequests: number;
  deniedRequests: number;
  bucketEvictions: number;
}

export class RateLimiter {
  private readonly capacity: number;
  private readonly refillPerSecond: number;
  private readonly maxBuckets: number;
  private readonly buckets = new Map<string, { tokens: number; updatedAt: number }>();
  private totalRequests = 0;
  private deniedRequests = 0;
  private bucketEvictions = 0;

  constructor(opts: { capacity: number; refillPerSecond: number; maxBuckets?: number }) {
    this.capacity = opts.capacity;
    this.refillPerSecond = opts.refillPerSecond;
    this.maxBuckets = Math.max(1, opts.maxBuckets ?? 10_000);
  }

  take(key: string, cost = 1): { allowed: boolean; state: RateLimitState } {
    this.totalRequests += 1;
    const now = Date.now();
    let bucket = this.buckets.get(key);
    if (!bucket) {
      this.evictOldestBucketIfFull();
      bucket = { tokens: this.capacity, updatedAt: now };
    }
    const elapsed = (now - bucket.updatedAt) / 1000;
    bucket.tokens = Math.min(this.capacity, bucket.tokens + elapsed * this.refillPerSecond);
    bucket.updatedAt = now;
    if (bucket.tokens < cost) {
      this.deniedRequests += 1;
      this.buckets.set(key, bucket);
      return {
        allowed: false,
        state: {
          tokensRemaining: Math.floor(bucket.tokens),
          resetAt: now + Math.ceil((cost - bucket.tokens) / this.refillPerSecond) * 1000,
        },
      };
    }
    bucket.tokens -= cost;
    this.buckets.set(key, bucket);
    return {
      allowed: true,
      state: { tokensRemaining: Math.floor(bucket.tokens), resetAt: now },
    };
  }

  reset(key: string): void {
    this.buckets.delete(key);
  }

  size(): number {
    return this.buckets.size;
  }

  /** Return aggregate counters without exposing actor identifiers. */
  metrics(): RateLimiterMetrics {
    return {
      activeBuckets: this.buckets.size,
      maxBuckets: this.maxBuckets,
      totalRequests: this.totalRequests,
      deniedRequests: this.deniedRequests,
      bucketEvictions: this.bucketEvictions,
    };
  }

  private evictOldestBucketIfFull(): void {
    if (this.buckets.size < this.maxBuckets) return;
    let oldestKey: string | undefined;
    let oldestUpdatedAt = Number.POSITIVE_INFINITY;
    for (const [key, bucket] of this.buckets) {
      if (bucket.updatedAt < oldestUpdatedAt) {
        oldestKey = key;
        oldestUpdatedAt = bucket.updatedAt;
      }
    }
    if (oldestKey !== undefined) {
      this.buckets.delete(oldestKey);
      this.bucketEvictions += 1;
    }
  }
}

export class Gateway {
  private readonly circuitBreakers = new Map<string, CircuitBreaker>();

  constructor(
    private readonly deps: {
      cache: ResponseCache;
      fallback: FallbackChain;
      rateLimiter: RateLimiter;
      router: Pick<LlmRouter, 'complete'>;
      circuitBreaker?: { failureThreshold?: number; cooldownMs?: number };
    },
  ) {}

  /** Return aggregate gateway metrics without exposing request content. */
  metrics(): { rateLimiter: RateLimiterMetrics; cacheEntries: number } {
    return {
      rateLimiter: this.deps.rateLimiter.metrics(),
      cacheEntries: this.deps.cache.size(),
    };
  }

  async complete(request: GatewayRequest, opts: { actorId: string; scopeId?: string } = { actorId: 'unscoped' }): Promise<GatewayResponse> {
    const rl = this.deps.rateLimiter.take(opts.actorId);
    if (!rl.allowed) {
      const err: Error & { statusCode?: number } = new Error('rate limit exceeded');
      err.statusCode = 429;
      throw err;
    }

    const cacheRequest = { ...request, scopeId: opts.scopeId ?? opts.actorId };
    const cacheHit = this.deps.cache.get(cacheRequest);
    if (cacheHit) {
      return {
        content: cacheHit.content,
        provider: cacheHit.provider,
        model: cacheHit.model,
        promptTokens: cacheHit.promptTokens,
        completionTokens: cacheHit.completionTokens,
        costUsd: cacheHit.costUsd,
        cacheHit: true,
        latencyMs: 0,
      };
    }

    const started = Date.now();
    let lastError: Error | undefined;
    const providers = [
      request.provider,
      ...this.deps.fallback.order().filter((provider) => provider !== request.provider),
    ];
    for (const provider of providers) {
      try {
        const breaker = this.circuitBreakers.get(provider) ?? new CircuitBreaker(
          `provider:${provider}`,
          this.deps.circuitBreaker?.failureThreshold,
          this.deps.circuitBreaker?.cooldownMs,
        );
        this.circuitBreakers.set(provider, breaker);
        const providerRequest = {
            prompt: request.prompt,
            model: request.model,
            temperature: request.temperature,
            provider,
            signal: request.signal,
            ...(provider === request.provider && request.baseUrl ? { baseUrl: request.baseUrl } : {}),
            ...(provider === request.provider && request.apiKey ? { apiKey: request.apiKey } : {}),
          };
        const result = await breaker.execute(() => this.deps.router.complete(providerRequest));
        const latencyMs = Date.now() - started;
        this.deps.cache.set({
          ...request,
          provider,
          content: result.content,
          promptTokens: result.promptTokens,
          completionTokens: result.completionTokens,
          costUsd: result.costUsd,
        }, cacheRequest);
        return { ...result, provider, latencyMs, cacheHit: false };
      } catch (err) {
        lastError = err as Error;
        // Continue to the next provider in the chain.
      }
    }
    throw lastError ?? new Error('all providers in fallback chain failed');
  }
}
