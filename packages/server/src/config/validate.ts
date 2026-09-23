import type { AppConfig } from '@promptsheon/shared';

export function validateConfig(config: AppConfig): void {
  const isProduction = config.server.nodeEnv === 'production';

  if (isProduction && !config.auth.enabled) {
    throw new Error('PROMPTSHEON_AUTH must be enabled in production');
  }
  if (config.auth.enabled && !config.auth.jwtSecret) {
    throw new Error('PROMPTSHEON_JWT_SECRET is required when auth is enabled');
  }
  if (config.server.port < 1 || config.server.port > 65535) {
    throw new Error('PROMPTSHEON_PORT must be between 1 and 65535');
  }
  if (!Number.isInteger(config.llm.maxRetries) || config.llm.maxRetries < 0) {
    throw new Error('PROMPTSHEON_LLM_MAX_RETRIES must be a non-negative integer');
  }
  if (!Number.isFinite(config.llm.timeoutMs) || config.llm.timeoutMs <= 0) {
    throw new Error('PROMPTSHEON_LLM_TIMEOUT_MS must be greater than zero');
  }
}
