import { describe, expect, it, vi } from 'vitest';
import { ConverseCommand } from '@aws-sdk/client-bedrock-runtime';
import { LlmRouter } from '../src/llm/router.js';

describe('LlmRouter', () => {
  it('completes through Bedrock Converse without requiring a live credential', async () => {
    const send = vi.fn().mockResolvedValue({
      output: { message: { content: [{ text: 'bedrock response' }] } },
    });
    const router = new LlmRouter(undefined, undefined, { send });

    const result = await router.complete({
      prompt: 'hello',
      model: 'amazon.nova-lite-v1:0',
      temperature: 0.2,
      provider: 'bedrock',
    });

    expect(result.content).toBe('bedrock response');
    expect(result.promptTokens).toBe(2);
    expect(result.completionTokens).toBe(4);
    expect(send).toHaveBeenCalledTimes(1);
    const command = send.mock.calls[0]?.[0];
    expect(command).toBeInstanceOf(ConverseCommand);
    expect(command.input).toMatchObject({
      modelId: 'amazon.nova-lite-v1:0',
      inferenceConfig: { temperature: 0.2, maxTokens: 1024 },
    });
  });

  it('fails clearly when Bedrock has no configured region', async () => {
    const router = new LlmRouter();

    await expect(router.complete({
      prompt: 'hello',
      model: 'amazon.nova-lite-v1:0',
      temperature: 0.2,
      provider: 'bedrock',
    })).rejects.toThrow('Bedrock region is required');
  });

  it('does not duplicate /v1 for OpenAI-compatible custom endpoints', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ data: [] }), { status: 200 }),
    );
    const router = new LlmRouter();

    await expect(router.probe({
      provider: 'custom',
      baseUrl: 'https://integrate.api.nvidia.com/v1',
      apiKey: 'test-key',
      model: 'meta/llama-3.1-8b-instruct',
    })).resolves.toMatchObject({ model: 'meta/llama-3.1-8b-instruct' });

    expect(fetchMock).toHaveBeenCalledWith(
      'https://integrate.api.nvidia.com/v1/models',
      expect.objectContaining({ headers: { Authorization: 'Bearer test-key' } }),
    );
    fetchMock.mockRestore();
  });

  it('uses provider-reported token usage when completion metadata is available', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({
        choices: [{ message: { content: 'provider response' } }],
        usage: { prompt_tokens: 41, completion_tokens: 17 },
      }), { status: 200 }),
    );
    const router = new LlmRouter();
    try {
      const result = await router.complete({
        prompt: 'short',
        model: 'provider-model',
        temperature: 0,
        provider: 'openai',
        apiKey: 'test-key',
        baseUrl: 'https://provider.example',
      });
      expect(result.promptTokens).toBe(41);
      expect(result.completionTokens).toBe(17);
    } finally {
      fetchMock.mockRestore();
    }
  });

  it('tolerates empty keep-alive frames in provider SSE streams', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response([
        ': keep-alive',
        '',
        'data: {"choices":[{"delta":{"content":"hello"}}]}',
        '',
        'data: [DONE]',
        '',
      ].join('\n'), { status: 200, headers: { 'content-type': 'text/event-stream' } }),
    );
    const router = new LlmRouter();
    try {
      const chunks = [];
      for await (const chunk of router.stream({
        prompt: 'hello',
        model: 'provider-model',
        temperature: 0,
        provider: 'openai',
        apiKey: 'test-key',
        baseUrl: 'https://provider.example',
      })) chunks.push(chunk);
      expect(chunks.map((chunk) => chunk.text).join('')).toBe('hello');
      expect(chunks.at(-1)?.done).toBe(true);
    } finally {
      fetchMock.mockRestore();
    }
  });

  it('redacts credentials returned in provider error bodies', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('apiKey=provider-secret', { status: 401 }),
    );
    const router = new LlmRouter();

    try {
      await router.probe({
        provider: 'custom',
        baseUrl: 'https://provider.example',
        apiKey: 'test-key',
        model: 'provider-model',
      });
      throw new Error('expected provider probe to fail');
    } catch (error) {
      expect(String(error)).toContain('Custom endpoint responded 401');
      expect(String(error)).not.toContain('provider-secret');
      expect(String(error)).toContain('apiKey=[REDACTED]');
    } finally {
      fetchMock.mockRestore();
    }
  });
});
