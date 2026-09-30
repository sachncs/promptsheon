import Fastify from 'fastify';
import { describe, expect, it } from 'vitest';
import { registerPlaygroundRoutes } from '../src/routes/playground.js';

describe('playground routes', () => {
  it('preserves provider connection fields across a sweep', async () => {
    const requests: Array<Record<string, unknown>> = [];
    const app = Fastify();
    await app.register(async (instance) => {
      registerPlaygroundRoutes(instance, {
        gateway: {
          complete: async (request) => {
            requests.push(request as unknown as Record<string, unknown>);
            return {
              content: 'ok',
              provider: 'custom',
              model: 'test-model',
              promptTokens: 1,
              completionTokens: 1,
              costUsd: 0,
              latencyMs: 1,
              cacheHit: false,
            };
          },
        },
      } as never);
    });

    const response = await app.inject({
      method: 'POST',
      url: '/api/playground/sweep',
      payload: {
        base: {
          prompt: 'base',
          model: 'test-model',
          provider: 'custom',
          baseUrl: 'https://llm.example.test',
          apiKey: 'test-key',
        },
        variants: [
          { prompt: 'first', temperature: 0.1 },
          { prompt: 'second', temperature: 0.9 },
        ],
      },
    });

    expect(response.statusCode).toBe(200);
    expect(requests).toHaveLength(2);
    expect(requests).toEqual(expect.arrayContaining([
      expect.objectContaining({ prompt: 'first', baseUrl: 'https://llm.example.test', apiKey: 'test-key' }),
      expect.objectContaining({ prompt: 'second', baseUrl: 'https://llm.example.test', apiKey: 'test-key' }),
    ]));
    await app.close();
  });
});
