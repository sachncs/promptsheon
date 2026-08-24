import { Agent, BedrockModel } from '@strands-agents/sdk';
import type { AppConfig } from '@promptsheon/shared';

export function createModel(config: AppConfig) {
  const provider = config.llm.defaultProvider;
  const modelId = config.llm.defaultModel;
  const baseUrl = config.llm.baseUrl;

  switch (provider) {
    case 'openai': {
      try {
        // eslint-disable-next-line @typescript-eslint/no-require-imports
        const { OpenAIModel } = require('@strands-agents/sdk/models/openai');
        return baseUrl
          ? new OpenAIModel({ modelId, baseURL: baseUrl })
          : new OpenAIModel({ modelId });
      } catch {
        return new BedrockModel({ modelId });
      }
    }
    case 'anthropic': {
      try {
        // eslint-disable-next-line @typescript-eslint/no-require-imports
        const { AnthropicModel } = require('@strands-agents/sdk/models/anthropic');
        return baseUrl
          ? new AnthropicModel({ modelId, baseURL: baseUrl })
          : new AnthropicModel({ modelId });
      } catch {
        return new BedrockModel({ modelId });
      }
    }
    case 'custom': {
      // Custom provider: the SDK must know the base URL. We try the
      // Anthropic SDK first because the MiniMax / most provider-agnostic
      // gateways expose an Anthropic-compatible surface. If the SDK
      // doesn't accept a baseURL, fall back to Bedrock and warn.
      try {
        // eslint-disable-next-line @typescript-eslint/no-require-imports
        const { AnthropicModel } = require('@strands-agents/sdk/models/anthropic');
        return new AnthropicModel({ modelId, baseURL: baseUrl });
      } catch {
        return new BedrockModel({ modelId });
      }
    }
    case 'bedrock':
    default:
      return new BedrockModel({ modelId });
  }
}
