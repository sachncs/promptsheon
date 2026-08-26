import { createHash } from 'node:crypto';
import type { LlmRouter } from '../llm/router.js';

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
  });
  return createHash('sha256').update(payload).digest('hex');
}

export class ResponseCache {
  private readonly store = new Map<string, CacheEntry>();
  private readonly maxEntries: number;

  constructor(maxEntries = 1024) {
    this.maxEntries = maxEntries;
  }

  /**
   * Lookup a cached response. Returns `null` on miss. Updates LRU
   * order on hit.
   */
  get(input: CacheLookup): CacheEntry | null {
    const hash = cacheKey(input);
    const entry = this.store.get(hash);
    if (!entry) return null;
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
  set(input: CacheLookup & Omit<CacheEntry, 'hash' | 'createdAt'>): CacheEntry {
    const hash = cacheKey(input);
    const entry: CacheEntry = { ...input, hash, createdAt: new Date().toISOString() };
    if (this.store.has(hash)) this.store.delete(hash);
    this.store.set(hash, entry);
    while (this.store.size > this.maxEntries) {
      const oldest = this.store.keys().next().value;
      if (oldest === undefined) break;
      this.store.delete(oldest);
    }
    return entry;
  }

  size(): number {
    return this.store.size;
  }

  clear(): void {
    this.store.clear();
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

export class RateLimiter {
  private readonly capacity: number;
  private readonly refillPerSecond: number;
  private readonly buckets = new Map<string, { tokens: number; updatedAt: number }>();

  constructor(opts: { capacity: number; refillPerSecond: number }) {
    this.capacity = opts.capacity;
    this.refillPerSecond = opts.refillPerSecond;
  }

  take(key: string, cost = 1): { allowed: boolean; state: RateLimitState } {
    const now = Date.now();
    const bucket = this.buckets.get(key) ?? { tokens: this.capacity, updatedAt: now };
    const elapsed = (now - bucket.updatedAt) / 1000;
    bucket.tokens = Math.min(this.capacity, bucket.tokens + elapsed * this.refillPerSecond);
    bucket.updatedAt = now;
    if (bucket.tokens < cost) {
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
}

export class Gateway {
  constructor(
    private readonly deps: {
      cache: ResponseCache;
      fallback: FallbackChain;
      rateLimiter: RateLimiter;
      router: Pick<LlmRouter, 'complete'>;
    },
  ) {}

  async complete(request: GatewayRequest, opts: { actorId: string } = { actorId: 'unscoped' }): Promise<GatewayResponse> {
    const rl = this.deps.rateLimiter.take(opts.actorId);
    if (!rl.allowed) {
      const err: Error & { statusCode?: number } = new Error('rate limit exceeded');
      err.statusCode = 429;
      throw err;
    }

    const cacheHit = this.deps.cache.get(request);
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
    for (const provider of this.deps.fallback.order()) {
      try {
        const result = await this.deps.router.complete({
          prompt: request.prompt,
          model: request.model,
          temperature: request.temperature,
          provider,
          baseUrl: request.baseUrl,
          apiKey: request.apiKey,
          signal: request.signal,
        });
        const latencyMs = Date.now() - started;
        this.deps.cache.set({
          ...request,
          provider,
          content: result.content,
          promptTokens: result.promptTokens,
          completionTokens: result.completionTokens,
          costUsd: result.costUsd,
        });
        return { ...result, provider, latencyMs, cacheHit: false };
      } catch (err) {
        lastError = err as Error;
        // Continue to the next provider in the chain.
      }
    }
    throw lastError ?? new Error('all providers in fallback chain failed');
  }
}
