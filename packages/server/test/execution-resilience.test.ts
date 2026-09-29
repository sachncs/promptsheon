import { describe, expect, it } from 'vitest';
import { ToolRegistry, type ToolAdapter } from '../src/application/execution-ports.js';
import { CircuitBreaker, CircuitOpenError } from '../src/application/execution-resilience.js';

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
});
