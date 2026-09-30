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
});
