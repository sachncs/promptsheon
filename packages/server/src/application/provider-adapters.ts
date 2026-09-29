import { z } from 'zod';
import type { LlmRouter } from '../llm/router.js';
import type { ModelAdapter, ModelRequest, ModelResponse } from './execution-ports.js';

const ProviderResponseSchema = z.strictObject({
  content: z.string(),
  promptTokens: z.number().int().nonnegative(),
  completionTokens: z.number().int().nonnegative(),
  costUsd: z.number().finite().nonnegative(),
  model: z.string().min(1),
});

/** Adapts the existing LLM router to the provider-neutral execution port. */
export class RouterModelAdapter implements ModelAdapter {
  constructor(
    private readonly router: Pick<LlmRouter, 'complete'>,
    readonly provider = '*',
  ) {}

  async invoke(request: ModelRequest): Promise<ModelResponse> {
    if (request.signal.aborted) throw new Error('model invocation cancelled');
    const response = ProviderResponseSchema.parse(await this.router.complete({
      prompt: `${request.systemPrompt}\n\n${request.input}`,
      model: request.model,
      temperature: request.temperature,
      provider: this.provider === '*' ? request.provider ?? 'custom' : this.provider,
      signal: request.signal,
    }));
    if (request.signal.aborted) throw new Error('model invocation cancelled');
    return {
      text: response.content,
      promptTokens: response.promptTokens,
      completionTokens: response.completionTokens,
      costUsd: response.costUsd,
    };
  }
}
