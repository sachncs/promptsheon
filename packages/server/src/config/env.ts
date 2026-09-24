import { readFileSync, existsSync } from 'node:fs';
import { join } from 'node:path';
import type { AppConfig } from '@promptsheon/shared';

/**
 * Load .env from the cwd into process.env (no-op if missing).
 * Existing process.env values take precedence so OS-level overrides win.
 */
function loadDotEnv(): void {
  const path = join(process.cwd(), '.env');
  if (!existsSync(path)) return;
  const raw = readFileSync(path, 'utf-8');
  for (const line of raw.split('\n')) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const eq = trimmed.indexOf('=');
    if (eq < 0) continue;
    const key = trimmed.slice(0, eq).trim();
    const value = trimmed.slice(eq + 1).trim().replace(/^['"]|['"]$/g, '');
    if (!(key in process.env)) process.env[key] = value;
  }
}

loadDotEnv();

function envString(key: string, fallback: string): string {
  return process.env[key] || fallback;
}

function envInt(key: string, fallback: number): number {
  const raw = process.env[key];
  if (!raw) return fallback;
  if (!/^-?\d+$/.test(raw.trim())) {
    throw new Error(`${key} must be an integer`);
  }
  const n = Number(raw);
  if (!Number.isSafeInteger(n)) {
    throw new Error(`${key} must be a safe integer`);
  }
  return n;
}

function envBool(key: string, fallback: boolean): boolean {
  const raw = process.env[key];
  if (!raw) return fallback;
  switch (raw.trim().toLowerCase()) {
    case '1':
    case 'true':
    case 'yes':
      return true;
    case '0':
    case 'false':
    case 'no':
      return false;
    default:
      throw new Error(`${key} must be a boolean`);
  }
}

export function loadConfig(): AppConfig {
  return {
    server: {
      port: envInt('PROMPTSHEON_PORT', 8080),
      host: envString('PROMPTSHEON_HOST', '127.0.0.1'),
      dbPath: envString('PROMPTSHEON_DB_PATH', 'promptsheon.db'),
      casPath: envString('PROMPTSHEON_CAS_PATH', '.promptsheon'),
      frontendPath: envString('PROMPTSHEON_FRONTEND_PATH', './frontend/.next'),
      corsOrigin: envString('PROMPTSHEON_CORS_ORIGIN', 'http://localhost:3000'),
      logLevel: envString('PROMPTSHEON_LOG_LEVEL', 'info'),
      nodeEnv: envString('PROMPTSHEON_NODE_ENV', envString('NODE_ENV', 'development')),
      fipsMode: envBool('PROMPTSHEON_FIPS_MODE', false),
      evalAllowedHosts: envString('PROMPTSHEON_EVAL_ALLOWED_HOSTS', '')
        .split(',')
        .map((host) => host.trim())
        .filter((host) => host.length > 0),
      allowPrivateNetworks: envString('PROMPTSHEON_NODE_ENV', envString('NODE_ENV', 'development')) !== 'production',
      e2eSessionEnabled: envBool('PROMPTSHEON_E2E', false),
      policyFile: process.env['PROMPTSHEON_POLICY_FILE'] || undefined,
      webhookSecret: process.env['PROMPTSHEON_WEBHOOK_SECRET'] || undefined,
      rateLimitMax: envInt('PROMPTSHEON_RATE_LIMIT_MAX', 100),
      otelEndpoint: process.env['PROMPTSHEON_OTEL_ENDPOINT'] || undefined,
    },
    llm: {
      defaultProvider: envString('PROMPTSHEON_LLM_PROVIDER', 'openai'),
      defaultModel: envString('PROMPTSHEON_LLM_MODEL', 'gpt-4'),
      apiKeyEnvVar: envString('PROMPTSHEON_LLM_API_KEY_ENV', 'OPENAI_API_KEY'),
      maxRetries: envInt('PROMPTSHEON_LLM_MAX_RETRIES', 5),
      timeoutMs: envInt('PROMPTSHEON_LLM_TIMEOUT_MS', 120000),
      baseUrl: process.env['LLM_BASE_URL'] || process.env['ANTHROPIC_BASE_URL'] || process.env['OPENAI_BASE_URL'] || undefined,
    },
    auth: {
      enabled: envBool('PROMPTSHEON_AUTH', false),
      jwtSecret: envString('PROMPTSHEON_JWT_SECRET', ''),
      scimBearerToken: envString('PROMPTSHEON_SCIM_TOKEN', ''),
    },
    selfEvolve: {
      enabled: envBool('PROMPTSHEON_SELF_EVOLVE_ENABLED', false),
      defaultCooldownSec: envInt('PROMPTSHEON_SELF_EVOLVE_COOLDOWN_SEC', 900),
      maxConcurrent: envInt('PROMPTSHEON_SELF_EVOLVE_MAX_CONCURRENT', 3),
    },
  };
}
