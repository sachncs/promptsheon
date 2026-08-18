import { Agent, BedrockModel } from '@strands-agents/sdk';
import type { AppConfig } from '@promptsheon/shared';

export function createModel(config: AppConfig) {
  const provider = config.llm.defaultProvider;
  const modelId = config.llm.defaultModel;

  switch (provider) {
    case 'openai': {
      try {
        // eslint-disable-next-line @typescript-eslint/no-require-imports
        const { OpenAIModel } = require('@strands-agents/sdk/models/openai');
        return new OpenAIModel({ modelId });
      } catch {
        return new BedrockModel({ modelId });
      }
    }
    case 'anthropic': {
      try {
        // eslint-disable-next-line @typescript-eslint/no-require-imports
        const { AnthropicModel } = require('@strands-agents/sdk/models/anthropic');
        return new AnthropicModel({ modelId });
      } catch {
        return new BedrockModel({ modelId });
      }
    }
    case 'bedrock':
    default:
      return new BedrockModel({ modelId });
  }
}
