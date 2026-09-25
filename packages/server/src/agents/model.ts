import { Agent, BedrockModel } from '@strands-agents/sdk';
import type { AppConfig } from '@promptsheon/shared';

export function createModel(config: AppConfig) {
  const provider = config.llm.defaultProvider;
  const modelId = config.llm.defaultModel;
  const baseUrl = config.llm.baseUrl;
  const credentials = config.llm.credentials;

  switch (provider) {
    case 'openai': {
      try {
        // eslint-disable-next-line @typescript-eslint/no-require-imports
        const { OpenAIModel } = require('@strands-agents/sdk/models/openai');
        return new OpenAIModel({
          modelId,
          ...(baseUrl ? { baseURL: baseUrl } : {}),
          ...(credentials?.openaiApiKey ? { apiKey: credentials.openaiApiKey } : {}),
        });
      } catch {
        return new BedrockModel({ modelId });
      }
    }
    case 'anthropic': {
      try {
        // eslint-disable-next-line @typescript-eslint/no-require-imports
        const { AnthropicModel } = require('@strands-agents/sdk/models/anthropic');
        return new AnthropicModel({
          modelId,
          ...(baseUrl ? { baseURL: baseUrl } : {}),
          ...(credentials?.anthropicApiKey ? { apiKey: credentials.anthropicApiKey } : {}),
        });
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
        return new AnthropicModel({
          modelId,
          baseURL: baseUrl,
          ...(credentials?.customApiKey ? { apiKey: credentials.customApiKey } : {}),
        });
      } catch {
        return new BedrockModel({ modelId });
      }
    }
    case 'bedrock':
    default:
      return new BedrockModel({
        modelId,
        ...(credentials?.bedrock
          ? {
              region: credentials.bedrock.region,
              clientConfig: {
                region: credentials.bedrock.region,
                credentials: {
                  accessKeyId: credentials.bedrock.accessKeyId,
                  secretAccessKey: credentials.bedrock.secretAccessKey,
                },
              },
            }
          : {}),
      });
  }
}
