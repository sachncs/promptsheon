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
  };
  llm: {
    defaultProvider: string;
    defaultModel: string;
    apiKeyEnvVar: string;
    maxRetries: number;
    timeoutMs: number;
    baseUrl?: string;
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
