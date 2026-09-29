import { describe, expect, it } from 'vitest';
import { Agent } from '@strands-agents/sdk';
import { SimulatedModel } from '../src/agents/simulated-model.js';
import { LlmProbeRequestSchema, LlmRouter } from '../src/llm/router.js';

describe('credential-free simulated provider', () => {
  it('invokes through the Strands Agent contract without credentials', async () => {
    const agent = new Agent({
      model: new SimulatedModel(),
      systemPrompt: 'You are a deterministic test agent.',
    });

    const result = await agent.invoke('summarise this fixture');
    expect(result.lastMessage.content).toEqual([
      { type: 'textBlock', text: '[simulation:promptsheon-simulator] summarise this fixture' },
    ]);
  });

  it('completes and probes without requiring an API key', async () => {
    const router = new LlmRouter();
    await expect(router.complete({
      provider: 'simulated',
      model: 'local-test',
      prompt: 'hello',
      temperature: 0,
    })).resolves.toMatchObject({
      content: '[simulation:local-test] hello',
      model: 'local-test',
    });

    expect(LlmProbeRequestSchema.parse({ provider: 'simulated', model: 'local-test' })).toMatchObject({
      provider: 'simulated',
    });
    await expect(router.probe({ provider: 'simulated', model: 'local-test' })).resolves.toMatchObject({
      skipped: true,
      skipReason: 'local deterministic simulator',
    });
  });
});
