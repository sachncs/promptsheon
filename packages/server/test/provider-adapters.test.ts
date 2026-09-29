import { describe, expect, it } from 'vitest';
import { RouterModelAdapter } from '../src/application/provider-adapters.js';

const request = (signal: AbortSignal) => ({ provider: 'openai', model: 'gpt-test', systemPrompt: 'system', input: 'input', maxOutputTokens: 100, temperature: 0, signal });

describe('RouterModelAdapter', () => {
  it('normalizes a valid provider response', async () => {
    const adapter = new RouterModelAdapter({ complete: async (input) => ({ content: `${input.provider}:ok`, promptTokens: 2, completionTokens: 3, costUsd: 0.01, model: input.model }) });
    await expect(adapter.invoke(request(new AbortController().signal))).resolves.toMatchObject({ text: 'openai:ok', promptTokens: 2, completionTokens: 3, costUsd: 0.01 });
  });

  it('rejects malformed provider responses', async () => {
    const adapter = new RouterModelAdapter({ complete: async () => ({ content: 42 } as never) });
    await expect(adapter.invoke(request(new AbortController().signal))).rejects.toThrow();
  });

  it('does not call a provider after cancellation', async () => {
    let calls = 0;
    const adapter = new RouterModelAdapter({ complete: async () => { calls++; throw new Error('should not call'); } });
    const controller = new AbortController();
    controller.abort();
    await expect(adapter.invoke(request(controller.signal))).rejects.toThrow('cancelled');
    expect(calls).toBe(0);
  });

  it('preserves provider rate-limit classification', async () => {
    const error = Object.assign(new Error('too many requests'), { statusCode: 429 });
    const adapter = new RouterModelAdapter({ complete: async () => { throw error; } });
    await expect(adapter.invoke(request(new AbortController().signal))).rejects.toMatchObject({ statusCode: 429 });
  });

  it('forwards provider timeout failures without converting them into success', async () => {
    const adapter = new RouterModelAdapter({ complete: async (input) => {
      expect(input.signal?.aborted).toBe(false);
      throw Object.assign(new Error('provider timeout'), { name: 'TimeoutError' });
    } });
    await expect(adapter.invoke(request(new AbortController().signal))).rejects.toMatchObject({ name: 'TimeoutError' });
  });
});
