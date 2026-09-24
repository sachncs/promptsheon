export interface LlmCredentials {
  openaiApiKey?: string;
  anthropicApiKey?: string;
  customApiKey?: string;
  bedrock?: {
    region: string;
    accessKeyId: string;
    secretAccessKey: string;
  };
}

export interface AppConfig {
  server: {
    port: number;
    host: string;
    dbPath: string;
    casPath: string;
    frontendPath: string;
    corsOrigin: string;
    logLevel: string;
    nodeEnv: string;
    fipsMode: boolean;
  };
  llm: {
    defaultProvider: string;
    defaultModel: string;
    apiKeyEnvVar: string;
    maxRetries: number;
    timeoutMs: number;
    baseUrl?: string;
    credentials?: LlmCredentials;
  };
  auth: {
    enabled: boolean;
    jwtSecret: string;
  };
  selfEvolve: {
    enabled: boolean;
    defaultCooldownSec: number;
    maxConcurrent: number;
  };
}
