import type { AppConfig } from '@promptsheon/shared';

const LOG_LEVELS = new Set(['fatal', 'error', 'warn', 'info', 'debug', 'trace', 'silent']);
const NODE_ENVIRONMENTS = new Set(['development', 'test', 'production']);

function validateOrigin(value: string): void {
  if (!value.trim()) return;

  let origin: URL;
  try {
    origin = new URL(value);
  } catch {
    throw new Error('PROMPTSHEON_CORS_ORIGIN must be a valid HTTP(S) origin');
  }

  if (
    !['http:', 'https:'].includes(origin.protocol)
    || origin.username
    || origin.password
    || origin.pathname !== '/'
    || origin.search
    || origin.hash
  ) {
    throw new Error('PROMPTSHEON_CORS_ORIGIN must be a valid HTTP(S) origin');
  }
}

export function validateConfig(config: AppConfig): void {
  const isProduction = config.server.nodeEnv === 'production';

  for (const [name, value] of Object.entries({
    PROMPTSHEON_HOST: config.server.host,
    PROMPTSHEON_DB_PATH: config.server.dbPath,
    PROMPTSHEON_CAS_PATH: config.server.casPath,
    PROMPTSHEON_FRONTEND_PATH: config.server.frontendPath,
    PROMPTSHEON_LLM_PROVIDER: config.llm.defaultProvider,
    PROMPTSHEON_LLM_MODEL: config.llm.defaultModel,
    PROMPTSHEON_LLM_API_KEY_ENV: config.llm.apiKeyEnvVar,
  })) {
    if (!value.trim()) throw new Error(`${name} must not be empty`);
  }

  if (isProduction && !config.auth.enabled) {
    throw new Error('PROMPTSHEON_AUTH must be enabled in production');
  }
  if (config.auth.enabled && config.auth.jwtSecret.length < 32) {
    throw new Error('PROMPTSHEON_JWT_SECRET must be at least 32 characters when auth is enabled');
  }
  if (isProduction && (!config.server.corsOrigin.trim() || config.server.corsOrigin.trim() === '*')) {
    throw new Error('PROMPTSHEON_CORS_ORIGIN must be an explicit origin in production');
  }
  if (!NODE_ENVIRONMENTS.has(config.server.nodeEnv)) {
    throw new Error('PROMPTSHEON_NODE_ENV must be one of development, test, or production');
  }
  if (!LOG_LEVELS.has(config.server.logLevel)) {
    throw new Error('PROMPTSHEON_LOG_LEVEL must be a valid structured log level');
  }
  validateOrigin(config.server.corsOrigin);
  if (config.llm.baseUrl !== undefined) {
    try {
      const baseUrl = new URL(config.llm.baseUrl);
      if (!['http:', 'https:'].includes(baseUrl.protocol)) throw new Error('invalid protocol');
    } catch {
      throw new Error('LLM_BASE_URL must be a valid HTTP(S) URL');
    }
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
  if (!Number.isInteger(config.selfEvolve.defaultCooldownSec) || config.selfEvolve.defaultCooldownSec < 0) {
    throw new Error('PROMPTSHEON_SELF_EVOLVE_COOLDOWN_SEC must be a non-negative integer');
  }
  if (!Number.isInteger(config.selfEvolve.maxConcurrent) || config.selfEvolve.maxConcurrent < 1) {
    throw new Error('PROMPTSHEON_SELF_EVOLVE_MAX_CONCURRENT must be a positive integer');
  }
}
