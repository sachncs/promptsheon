export interface AppConfig {
  server: {
    port: number;
    host: string;
    dbPath: string;
    casPath: string;
    frontendPath: string;
    corsOrigin: string;
    logLevel: string;
  };
  llm: {
    defaultProvider: string;
    defaultModel: string;
    apiKeyEnvVar: string;
    maxRetries: number;
    timeoutMs: number;
  };
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
