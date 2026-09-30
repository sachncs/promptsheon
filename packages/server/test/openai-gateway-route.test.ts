import Fastify from 'fastify';
import { describe, expect, it, vi } from 'vitest';
import { registerOpenAiGatewayRoutes } from '../src/routes/openai-gateway.js';

function gateway(content = 'simulated response') {
  return {
    complete: vi.fn(async (request: { prompt: string; model: string; provider: string }) => ({
      content,
      provider: request.provider,
      model: request.model,
      promptTokens: 4,
      completionTokens: 3,
      costUsd: 0,
      cacheHit: false,
      latencyMs: 1,
    })),
  } as never;
}

describe('OpenAI-compatible gateway route', () => {
  it('maps chat messages to a validated gateway request', async () => {
    const app = Fastify();
    const gw = gateway();
    registerOpenAiGatewayRoutes(app, { gateway: gw });

    const response = await app.inject({
      method: 'POST',
      url: '/v1/chat/completions',
      payload: {
        model: 'promptsheon-simulator',
        provider: 'simulated',
        messages: [{ role: 'user', content: 'hello' }],
      },
    });

    expect(response.statusCode).toBe(200);
    expect(gw.complete).toHaveBeenCalledWith(
      expect.objectContaining({ prompt: 'user: hello', model: 'promptsheon-simulator', provider: 'simulated' }),
      expect.objectContaining({ actorId: 'unscoped', scopeId: 'unscoped:unscoped' }),
    );
    expect(response.json()).toMatchObject({
      object: 'chat.completion',
      choices: [{ message: { role: 'assistant', content: 'simulated response' } }],
    });
  });

  it('returns an OpenAI-compatible SSE response for streamed requests', async () => {
    const app = Fastify();
    registerOpenAiGatewayRoutes(app, { gateway: gateway('streamed') });

    const response = await app.inject({
      method: 'POST',
      url: '/v1/chat/completions',
      payload: {
        model: 'promptsheon-simulator',
        provider: 'simulated',
        stream: true,
        messages: [{ role: 'user', content: [{ type: 'text', text: 'hello' }] }],
      },
    });

    expect(response.statusCode).toBe(200);
    expect(response.headers['content-type']).toContain('text/event-stream');
    expect(response.body).toContain('data: [DONE]');
    expect(response.body).toContain('streamed');
  });

  it('rejects malformed requests before invoking the gateway', async () => {
    const app = Fastify();
    const gw = gateway();
    registerOpenAiGatewayRoutes(app, { gateway: gw });

    const response = await app.inject({ method: 'POST', url: '/v1/chat/completions', payload: { model: 'm' } });

    expect(response.statusCode).toBe(422);
    expect(gw.complete).not.toHaveBeenCalled();
  });
});
