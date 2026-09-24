import { describe, it, expect } from 'vitest';
import { validateConfig } from '../src/config/validate.js';
import { loadConfig } from '../src/config/env.js';
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
      auth: { enabled: true, jwtSecret: 'a-real-secret-with-at-least-32-characters' },
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

  it('rejects weak authentication secrets', () => {
    expect(() => validateConfig({
      ...baseConfig,
      auth: { enabled: true, jwtSecret: 'short-secret' },
    })).toThrow(/at least 32 characters/);
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

  it('requires an explicit production CORS origin', () => {
    expect(() => validateConfig({
      ...baseConfig,
      server: { ...baseConfig.server, nodeEnv: 'production', corsOrigin: '*' },
      auth: { enabled: true, jwtSecret: 'a-real-secret-with-at-least-32-characters' },
    })).toThrow(/PROMPTSHEON_CORS_ORIGIN/);
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
    expect(() => validateConfig({
      ...baseConfig,
      selfEvolve: { ...baseConfig.selfEvolve, maxConcurrent: 0 },
    })).toThrow(/PROMPTSHEON_SELF_EVOLVE_MAX_CONCURRENT/);
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

describe('loadConfig environment parsing', () => {
  function withEnvironment(key: string, value: string, assertion: () => void): void {
    const previous = process.env[key];
    process.env[key] = value;
    try {
      assertion();
    } finally {
      if (previous === undefined) delete process.env[key];
      else process.env[key] = previous;
    }
  }

  it('fails fast on malformed numeric environment values', () => {
    withEnvironment('PROMPTSHEON_PORT', 'not-a-port', () => {
      expect(() => loadConfig()).toThrow(/PROMPTSHEON_PORT must be an integer/);
    });
  });

  it('fails fast on malformed boolean environment values', () => {
    withEnvironment('PROMPTSHEON_AUTH', 'sometimes', () => {
      expect(() => loadConfig()).toThrow(/PROMPTSHEON_AUTH must be a boolean/);
    });
  });
});
