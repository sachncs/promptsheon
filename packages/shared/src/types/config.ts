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
    /** Hosts permitted for outbound evaluation requests. */
    evalAllowedHosts?: string[];
    /** Whether evaluation requests may target private network addresses. */
    allowPrivateNetworks?: boolean;
    /** Enable the dedicated end-to-end browser session helper. */
    e2eSessionEnabled?: boolean;
    /** Optional Cedar policy file loaded during startup. */
    policyFile?: string;
    /** Webhook signing secret resolved during startup. */
    webhookSecret?: string;
    /** Maximum requests per minute per rate-limit key. */
    rateLimitMax?: number;
    /** Optional OpenTelemetry exporter endpoint. */
    otelEndpoint?: string;
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
    /** Optional SCIM bearer token, loaded only at the composition root. */
    scimBearerToken?: string;
  };
  selfEvolve: {
    enabled: boolean;
    defaultCooldownSec: number;
    maxConcurrent: number;
  };
}
