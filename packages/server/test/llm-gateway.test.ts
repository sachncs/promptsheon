import { describe, it, expect } from 'vitest';
import { ResponseCache, FallbackChain, Gateway, RateLimiter, cacheKey } from '../src/llm/gateway.js';
import type { LlmCompleteResult, LlmRouter } from '../src/llm/router.js';

function stubRouter(behaviour: (provider: string) => Promise<LlmCompleteResult>) {
  return {
    async complete(req: Parameters<LlmRouter['complete']>[0]): Promise<LlmCompleteResult> {
      return behaviour(req.provider);
    },
  } as unknown as Pick<LlmRouter, 'complete'>;
}

function okResult(provider: string, content = `hello from ${provider}`): LlmCompleteResult {
  return {
    content,
    promptTokens: 12,
    completionTokens: 21,
    costUsd: 0.001,
    model: 'gpt-4',
  };
}

describe('ResponseCache', () => {
  it('caches identical requests and returns the cached entry', async () => {
    const cache = new ResponseCache();
    const request = { prompt: 'hi', model: 'gpt-4', temperature: 0.7, provider: 'openai' };
    expect(cache.get(request)).toBeNull();
    const set = cache.set({ ...request, content: 'hello', promptTokens: 1, completionTokens: 1, costUsd: 0, model: 'gpt-4', provider: 'openai' });
    const got = cache.get(request);
    expect(got?.content).toBe('hello');
    expect(got?.hash).toBe(set.hash);
  });

  it('produces the same hash for equivalent inputs regardless of property order', () => {
    const a = cacheKey({ prompt: 'hi', model: 'm', temperature: 0.5, provider: 'p' });
    const b = cacheKey({ temperature: 0.5, provider: 'p', model: 'm', prompt: 'hi' });
    expect(a).toBe(b);
  });

  it('different temperatures produce different hashes', () => {
    const a = cacheKey({ prompt: 'hi', model: 'm', temperature: 0.5, provider: 'p' });
    const b = cacheKey({ prompt: 'hi', model: 'm', temperature: 0.7, provider: 'p' });
    expect(a).not.toBe(b);
  });

  it('isolates entries by provider and custom endpoint', () => {
    const simulator = cacheKey({ prompt: 'hi', model: 'm', temperature: 0.5, provider: 'simulated' });
    const openai = cacheKey({ prompt: 'hi', model: 'm', temperature: 0.5, provider: 'openai' });
    const customA = cacheKey({ prompt: 'hi', model: 'm', temperature: 0.5, provider: 'custom', baseUrl: 'https://one.example/v1' });
    const customB = cacheKey({ prompt: 'hi', model: 'm', temperature: 0.5, provider: 'custom', baseUrl: 'https://two.example/v1' });
    expect(simulator).not.toBe(openai);
    expect(customA).not.toBe(customB);
  });

  it('evicts least-recently-used entries past capacity', () => {
    const cache = new ResponseCache(2);
    cache.set({ prompt: 'a', model: 'm', temperature: 0, provider: 'p', content: '', promptTokens: 0, completionTokens: 0, costUsd: 0, model: 'm', provider: 'p' });
    cache.set({ prompt: 'b', model: 'm', temperature: 0, provider: 'p', content: '', promptTokens: 0, completionTokens: 0, costUsd: 0, model: 'm', provider: 'p' });
    cache.set({ prompt: 'c', model: 'm', temperature: 0, provider: 'p', content: '', promptTokens: 0, completionTokens: 0, costUsd: 0, model: 'm', provider: 'p' });
    expect(cache.get({ prompt: 'a', model: 'm', temperature: 0, provider: 'p' })).toBeNull();
    expect(cache.get({ prompt: 'b', model: 'm', temperature: 0, provider: 'p' })?.hash).toBeTruthy();
    expect(cache.get({ prompt: 'c', model: 'm', temperature: 0, provider: 'p' })?.hash).toBeTruthy();
  });
});

describe('RateLimiter', () => {
  it('refills tokens over time', async () => {
    const rl = new RateLimiter({ capacity: 2, refillPerSecond: 100 });
    // Drain the bucket.
    const a = rl.take('user-1', 2);
    expect(a.allowed).toBe(true);
    // Wait for refill, then take again — at 100/s refill, 30ms
    // gives back ~3 tokens which is capped at capacity=2, so the
    // post-deduction remainder is 0 but the call is allowed.
    await new Promise((r) => setTimeout(r, 30));
    const b = rl.take('user-1', 1);
    expect(b.allowed).toBe(true);
  });

  it('isolates buckets by key', () => {
    const rl = new RateLimiter({ capacity: 1, refillPerSecond: 0 });
    rl.take('a', 1);
    const b = rl.take('b', 1);
    expect(b.allowed).toBe(true);
    expect(b.state.tokensRemaining).toBe(0);
    const c = rl.take('a', 1);
    expect(c.allowed).toBe(false);
  });

  it('bounds the number of actor buckets and evicts the least recently updated bucket', () => {
    const rl = new RateLimiter({ capacity: 1, refillPerSecond: 0, maxBuckets: 2 });
    rl.take('oldest', 1);
    rl.take('newer', 1);
    expect(rl.size()).toBe(2);

    rl.take('latest', 1);

    expect(rl.size()).toBe(2);
    expect(rl.take('oldest', 1).allowed).toBe(true);
  });

  it('reports aggregate bucket churn and denied requests without actor keys', () => {
    const rl = new RateLimiter({ capacity: 1, refillPerSecond: 0, maxBuckets: 1 });
    rl.take('first', 1);
    expect(rl.take('first', 1).allowed).toBe(false);
    rl.take('second', 1);

    expect(rl.metrics()).toEqual({
      activeBuckets: 1,
      maxBuckets: 1,
      totalRequests: 3,
      deniedRequests: 1,
      bucketEvictions: 1,
    });
    expect(JSON.stringify(rl.metrics())).not.toContain('first');
  });
});

describe('Gateway', () => {
  it('honours the requested simulator before configured real-provider fallbacks', async () => {
    const providers: string[] = [];
    const gw = new Gateway({
      cache: new ResponseCache(),
      fallback: new FallbackChain(['custom', 'anthropic', 'openai']),
      rateLimiter: new RateLimiter({ capacity: 100, refillPerSecond: 100 }),
      router: stubRouter(async (provider) => {
        providers.push(provider);
        return okResult(provider, '[simulation] hello');
      }),
    });

    const result = await gw.complete({
      prompt: 'hello', model: 'promptsheon-simulator', temperature: 0.2, provider: 'simulated',
    });

    expect(result.provider).toBe('simulated');
    expect(providers).toEqual(['simulated']);
  });

  it('returns a cached entry without calling the router', async () => {
    const cache = new ResponseCache();
    cache.set({
      prompt: 'hi', model: 'gpt-4', temperature: 0, provider: 'openai',
      content: 'cached hello', promptTokens: 1, completionTokens: 1, costUsd: 0,
      model: 'gpt-4', provider: 'openai',
    });
    let routerCalls = 0;
    const router = stubRouter(async () => {
      routerCalls += 1;
      return okResult('openai');
    });
    const gw = new Gateway({
      cache,
      fallback: new FallbackChain(['openai']),
      rateLimiter: new RateLimiter({ capacity: 100, refillPerSecond: 100 }),
      router,
    });
    const out = await gw.complete({ prompt: 'hi', model: 'gpt-4', temperature: 0, provider: 'openai' });
    expect(out.content).toBe('cached hello');
    expect(out.cacheHit).toBe(true);
    expect(routerCalls).toBe(0);
  });

  it('falls through the chain on provider failure and caches the winner', async () => {
    const cache = new ResponseCache();
    const router = stubRouter(async (provider) => {
      if (provider === 'openai') throw new Error('openai down');
      return okResult(provider);
    });
    const gw = new Gateway({
      cache,
      fallback: new FallbackChain(['openai', 'anthropic', 'custom']),
      rateLimiter: new RateLimiter({ capacity: 100, refillPerSecond: 100 }),
      router,
    });
    const out = await gw.complete({ prompt: 'p', model: 'm', temperature: 0, provider: 'openai' });
    expect(out.provider).toBe('anthropic');
    expect(out.cacheHit).toBe(false);
    // Second call hits the cache from anthropic's perspective.
    const again = await gw.complete({ prompt: 'p', model: 'm', temperature: 0, provider: 'openai' });
    expect(again.provider).toBe('anthropic');
    expect(again.cacheHit).toBe(true);
  });

  it('throws when all providers in the chain fail', async () => {
    const router = stubRouter(async () => {
      throw new Error('down');
    });
    const gw = new Gateway({
      cache: new ResponseCache(),
      fallback: new FallbackChain(['openai', 'anthropic']),
      rateLimiter: new RateLimiter({ capacity: 100, refillPerSecond: 100 }),
      router,
    });
    await expect(
      gw.complete({ prompt: 'p', model: 'm', temperature: 0, provider: 'openai' }),
    ).rejects.toThrow(/down/);
  });

  it('throws 429 on rate-limit exhaustion', async () => {
    const router = stubRouter(async (p) => okResult(p));
    const gw = new Gateway({
      cache: new ResponseCache(),
      fallback: new FallbackChain(['openai']),
      rateLimiter: new RateLimiter({ capacity: 0, refillPerSecond: 0 }),
      router,
    });
    await expect(
      gw.complete({ prompt: 'p', model: 'm', temperature: 0, provider: 'openai' }, { actorId: 'a' }),
    ).rejects.toMatchObject({ statusCode: 429 });
  });

  it('opens a failing provider circuit before the next upstream call', async () => {
    let calls = 0;
    const gw = new Gateway({
      cache: new ResponseCache(),
      fallback: new FallbackChain(['openai']),
      rateLimiter: new RateLimiter({ capacity: 100, refillPerSecond: 100 }),
      router: stubRouter(async () => { calls += 1; throw new Error('provider down'); }),
      circuitBreaker: { failureThreshold: 2, cooldownMs: 60_000 },
    });
    const request = { prompt: 'circuit', model: 'm', temperature: 0, provider: 'openai' };
    await expect(gw.complete(request)).rejects.toThrow('provider down');
    await expect(gw.complete(request)).rejects.toThrow('provider down');
    await expect(gw.complete(request)).rejects.toThrow('circuit open: provider:openai');
    expect(calls).toBe(2);
  });
});

describe('FallbackChain', () => {
  it('preserves declared order', () => {
    const chain = new FallbackChain(['openai', 'anthropic', 'bedrock']);
    expect(chain.order()).toEqual(['openai', 'anthropic', 'bedrock']);
  });

  it('rejects empty chain', () => {
    expect(() => new FallbackChain([])).toThrow();
  });
});
