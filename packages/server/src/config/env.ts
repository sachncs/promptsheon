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
  const n = Number.parseInt(raw, 10);
  if (Number.isNaN(n)) return fallback;
  return n;
}

function envBool(key: string, fallback: boolean): boolean {
  const raw = process.env[key];
  if (!raw) return fallback;
  return raw === '1' || raw === 'true' || raw === 'yes';
}

export function loadConfig(): AppConfig {
  return {
    server: {
      port: envInt('PROMPTSHEON_PORT', 8080),
      host: envString('PROMPTSHEON_HOST', '127.0.0.1'),
      dbPath: envString('PROMPTSHEON_DB_PATH', 'promptsheon.db'),
      casPath: envString('PROMPTSHEON_CAS_PATH', '.promptsheon'),
      frontendPath: envString('PROMPTSHEON_FRONTEND_PATH', './frontend/dist'),
      corsOrigin: envString('PROMPTSHEON_CORS_ORIGIN', ''),
      logLevel: envString('PROMPTSHEON_LOG_LEVEL', 'info'),
      nodeEnv: envString('PROMPTSHEON_NODE_ENV', envString('NODE_ENV', 'development')),
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
      enabled: envBool('PROMPTSHEON_AUTH', true),
      jwtSecret: envString('PROMPTSHEON_JWT_SECRET', ''),
    },
    selfEvolve: {
      enabled: envBool('PROMPTSHEON_SELF_EVOLVE_ENABLED', false),
      defaultCooldownSec: envInt('PROMPTSHEON_SELF_EVOLVE_COOLDOWN_SEC', 900),
      maxConcurrent: envInt('PROMPTSHEON_SELF_EVOLVE_MAX_CONCURRENT', 3),
    },
  };
}
