import { describe, expect, it } from 'vitest';
import { ToolRegistry, type ToolAdapter } from '../src/application/execution-ports.js';
import { CircuitBreaker, CircuitOpenError, ConcurrencyLimiter } from '../src/application/execution-resilience.js';

const tool: ToolAdapter = {
  name: 'search', description: 'Search', inputSchema: {},
  invoke: async (input) => input['query'],
};

describe('execution resilience ports', () => {
  it('authorizes tool invocation and rejects denied access', async () => {
    const registry = new ToolRegistry(1);
    registry.register(tool);
    const context = { organizationId: 'org1', executionId: 'run1', signal: new AbortController().signal };
    await expect(registry.invoke('search', { query: 'hello' }, context, { authorize: () => true })).resolves.toBe('hello');
    await expect(registry.invoke('search', {}, context, { authorize: () => false })).rejects.toThrow('permission denied');
  });

  it('opens after threshold and recovers after cooldown', async () => {
    let clock = 0;
    const breaker = new CircuitBreaker('provider:one', 2, 100, () => clock);
    await expect(breaker.execute(async () => { throw new Error('one'); })).rejects.toThrow('one');
    await expect(breaker.execute(async () => { throw new Error('two'); })).rejects.toThrow('two');
    await expect(breaker.execute(async () => 'blocked')).rejects.toBeInstanceOf(CircuitOpenError);
    clock = 100;
    await expect(breaker.execute(async () => 'recovered')).resolves.toBe('recovered');
  });

  it('bounds concurrent work and releases capacity after completion', async () => {
    const limiter = new ConcurrencyLimiter(1);
    let release!: () => void;
    const first = limiter.run(() => new Promise<string>((resolve) => { release = () => resolve('first'); }));
    await Promise.resolve();
    const second = limiter.run(async () => 'second');
    expect(limiter.active).toBe(1);
    expect(limiter.queued).toBe(1);
    release();
    await expect(first).resolves.toBe('first');
    await expect(second).resolves.toBe('second');
    expect(limiter.active).toBe(0);
    expect(limiter.queued).toBe(0);
  });

  it('cancels a queued operation without consuming capacity', async () => {
    const limiter = new ConcurrencyLimiter(1);
    let release!: () => void;
    const first = limiter.run(() => new Promise<void>((resolve) => { release = resolve; }));
    const controller = new AbortController();
    const queued = limiter.run(async () => 'never', controller.signal);
    controller.abort();
    await expect(queued).rejects.toThrow('concurrency wait cancelled');
    expect(limiter.queued).toBe(0);
    release();
    await first;
    expect(limiter.active).toBe(0);
  });
});
