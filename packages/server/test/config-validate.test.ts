import { describe, it, expect } from 'vitest';
import { validateConfig } from '../src/config/validate.js';
import { resolveScimBearerToken } from '../src/routes/index.js';
import type { AppConfig } from '@promptsheon/shared';

const baseConfig: AppConfig = {
  server: {
    port: 8080,
    host: '127.0.0.1',
    dbPath: 'test.db',
    casPath: '.test-cas',
    frontendPath: './frontend/dist',
    corsOrigin: '',
    logLevel: 'info',
    nodeEnv: 'test',
    fipsMode: false,
  },
  llm: {
    defaultProvider: 'openai',
    defaultModel: 'gpt-4',
    apiKeyEnvVar: 'OPENAI_API_KEY',
    maxRetries: 5,
    timeoutMs: 120000,
  },
  auth: { enabled: false, jwtSecret: '' },
  selfEvolve: { enabled: false, defaultCooldownSec: 900, maxConcurrent: 3 },
};

describe('validateConfig (issue #47 — boot-time validation gate)', () => {
  it('passes when auth is disabled and the port is in range', () => {
    expect(() => validateConfig(baseConfig)).not.toThrow();
  });

  it('passes when auth is enabled and jwtSecret is set', () => {
    const config: AppConfig = {
      ...baseConfig,
      auth: { enabled: true, jwtSecret: 'a-real-secret' },
    };
    expect(() => validateConfig(config)).not.toThrow();
  });

  it('throws when auth is enabled and jwtSecret is empty', () => {
    const config: AppConfig = {
      ...baseConfig,
      auth: { enabled: true, jwtSecret: '' },
    };
    expect(() => validateConfig(config)).toThrow(/PROMPTSHEON_JWT_SECRET/);
  });

  it('throws when port is below 1', () => {
    const config: AppConfig = {
      ...baseConfig,
      server: { ...baseConfig.server, port: 0 },
    };
    expect(() => validateConfig(config)).toThrow(/PROMPTSHEON_PORT/);
  });

  it('throws when port is above 65535', () => {
    const config: AppConfig = {
      ...baseConfig,
      server: { ...baseConfig.server, port: 70000 },
    };
    expect(() => validateConfig(config)).toThrow(/PROMPTSHEON_PORT/);
  });

  it('refuses to boot production with authentication disabled', () => {
    const config: AppConfig = {
      ...baseConfig,
      server: { ...baseConfig.server, nodeEnv: 'production' },
    };
    expect(() => validateConfig(config)).toThrow(/PROMPTSHEON_AUTH/);
  });

  it('rejects invalid LLM retry and timeout settings', () => {
    expect(() => validateConfig({
      ...baseConfig,
      llm: { ...baseConfig.llm, maxRetries: -1 },
    })).toThrow(/PROMPTSHEON_LLM_MAX_RETRIES/);
    expect(() => validateConfig({
      ...baseConfig,
      llm: { ...baseConfig.llm, timeoutMs: 0 },
    })).toThrow(/PROMPTSHEON_LLM_TIMEOUT_MS/);
  });
});

describe('resolveScimBearerToken', () => {
  it('uses an explicitly configured token', () => {
    expect(resolveScimBearerToken('production', 'configured-token')).toBe('configured-token');
  });

  it('keeps a local development fallback', () => {
    expect(resolveScimBearerToken('development', undefined)).toBe('dev-scim-token');
  });

  it('fails closed in production when the token is missing', () => {
    expect(() => resolveScimBearerToken('production', undefined)).toThrow('PROMPTSHEON_SCIM_TOKEN is required in production');
  });
});
