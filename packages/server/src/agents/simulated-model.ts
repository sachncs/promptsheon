import { Model } from '@strands-agents/sdk';
import type { Message } from '@strands-agents/sdk';
import type { BaseModelConfig, ModelStreamEvent, StreamOptions } from '@strands-agents/sdk';

interface SimulatedModelConfig extends BaseModelConfig {
  modelId: string;
}

/** Deterministic, credential-free model for local development and tests. */
export class SimulatedModel extends Model<SimulatedModelConfig> {
  private config: SimulatedModelConfig;

  constructor(modelId = 'promptsheon-simulator') {
    super();
    this.config = { modelId, maxTokens: 512, temperature: 0 };
  }

  updateConfig(modelConfig: SimulatedModelConfig): void {
    this.config = { ...this.config, ...modelConfig };
  }

  getConfig(): SimulatedModelConfig {
    return { ...this.config };
  }

  async *stream(messages: Message[], options?: StreamOptions): AsyncIterable<ModelStreamEvent> {
    if (options?.cancelSignal?.aborted) throw new Error('simulated model invocation cancelled');
    const input = latestUserText(messages);
    const content = `[simulation:${this.config.modelId}] ${input || 'No user input supplied.'}`;
    const inputTokens = Math.ceil(JSON.stringify(messages).length / 4);
    const outputTokens = Math.ceil(content.length / 4);

    yield { type: 'modelMessageStartEvent', role: 'assistant' };
    yield { type: 'modelContentBlockStartEvent' };
    yield { type: 'modelContentBlockDeltaEvent', delta: { type: 'textDelta', text: content } };
    yield { type: 'modelContentBlockStopEvent' };
    yield { type: 'modelMessageStopEvent', stopReason: 'end_turn' };
    yield {
      type: 'modelMetadataEvent',
      usage: { inputTokens, outputTokens, totalTokens: inputTokens + outputTokens },
      metrics: { latencyMs: 0, timeToFirstByteMs: 0 },
    };
  }
}

function latestUserText(messages: Message[]): string {
  const message = [...messages].reverse().find((candidate) => candidate.role === 'user');
  if (!message) return '';
  return message.content
    .flatMap((block) => 'text' in block && typeof block.text === 'string' ? [block.text] : [])
    .join(' ')
    .trim();
}
