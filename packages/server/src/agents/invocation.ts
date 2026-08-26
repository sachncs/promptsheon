import { Agent } from '@strands-agents/sdk';
import type { AppConfig, Execution } from '@promptsheon/shared';
import { createModel } from './model.js';
import { extractText } from './utils.js';

export class InvocationAgent {
  private agent: Agent;

  constructor(config: AppConfig) {
    this.agent = new Agent({
      model: createModel(config),
      systemPrompt: `You are a capability execution engine. When invoked:
1. Look up the capability manifest
2. Execute the prompt with the provided inputs
3. Record the execution result
4. Return the outputs`,
    });
  }

  async invoke(
    capabilityVersionId: string,
    inputs: Record<string, unknown>,
    options: { environment?: string; traceId?: string } = {},
  ): Promise<Execution> {
    const startTime = Date.now();
    const result = await this.agent.invoke(JSON.stringify({
      action: 'invoke',
      capabilityVersionId,
      inputs,
      environment: options.environment ?? '',
      traceId: options.traceId ?? '',
    }));

    return {
      id: crypto.randomUUID(),
      capabilityVersionId,
      timestamp: new Date().toISOString(),
      inputs: JSON.stringify(inputs),
      outputs: extractText(result),
      model: '',
      provider: '',
      latencyMs: Date.now() - startTime,
      costUsd: 0,
      promptTokens: result.metrics?.accumulatedUsage?.inputTokens ?? 0,
      completionTokens: result.metrics?.accumulatedUsage?.outputTokens ?? 0,
      totalTokens: result.metrics?.accumulatedUsage?.totalTokens ?? 0,
      error: '',
      traceId: options.traceId ?? '',
      environment: options.environment ?? '',
      replayOf: null,
      replayCount: 0,
      inputHash: null,
    };
  }
}