export interface ServerConfig {
  port: number;
  host: string;
  dbPath: string;
  casPath: string;
  frontendPath: string;
  corsOrigin: string;
  logLevel: string;
}

export interface LlmConfig {
  defaultProvider: string;
  defaultModel: string;
  apiKeyEnvVar: string;
  maxRetries: number;
  timeoutMs: number;
}

export interface AppConfig {
  server: ServerConfig;
  llm: LlmConfig;
  auth: {
    enabled: boolean;
    jwtSecret: string;
  };
  selfEvolve: {
    enabled: boolean;
    defaultCooldownSec: number;
    maxConcurrentCycles: number;
  };
}
