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

  it('rejects E2E session issuance in production', () => {
    expect(() => validateConfig({
      ...baseConfig,
      server: { ...baseConfig.server, nodeEnv: 'production', corsOrigin: 'https://console.example.com', e2eSessionEnabled: true },
      auth: { enabled: true, jwtSecret: 'a-real-secret-with-at-least-32-characters' },
    })).toThrow(/PROMPTSHEON_E2E/);
  });

  it('rejects private-network evaluation in production', () => {
    expect(() => validateConfig({
      ...baseConfig,
      server: { ...baseConfig.server, nodeEnv: 'production', corsOrigin: 'https://console.example.com', allowPrivateNetworks: true },
      auth: { enabled: true, jwtSecret: 'a-real-secret-with-at-least-32-characters' },
    })).toThrow(/PROMPTSHEON_ALLOW_PRIVATE_NETWORKS/);
  });

  it('requires a positive request rate limit', () => {
    expect(() => validateConfig({
      ...baseConfig,
      server: { ...baseConfig.server, rateLimitMax: 0 },
    })).toThrow(/PROMPTSHEON_RATE_LIMIT_MAX/);
  });

  it('rejects invalid runtime environment and log level values', () => {
    expect(() => validateConfig({
      ...baseConfig,
      server: { ...baseConfig.server, nodeEnv: 'staging' },
    })).toThrow(/PROMPTSHEON_NODE_ENV/);
    expect(() => validateConfig({
      ...baseConfig,
      server: { ...baseConfig.server, logLevel: 'verbose' },
    })).toThrow(/PROMPTSHEON_LOG_LEVEL/);
  });

  it('rejects malformed CORS origins and LLM base URLs', () => {
    expect(() => validateConfig({
      ...baseConfig,
      server: { ...baseConfig.server, corsOrigin: 'not-an-origin' },
    })).toThrow(/PROMPTSHEON_CORS_ORIGIN/);
    expect(() => validateConfig({
      ...baseConfig,
      llm: { ...baseConfig.llm, baseUrl: 'ftp://llm.internal' },
    })).toThrow(/LLM_BASE_URL/);
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

  it('derives the development CORS origin from the frontend port', () => {
    const previousOrigin = process.env['PROMPTSHEON_CORS_ORIGIN'];
    const previousFrontendPort = process.env['PROMPTSHEON_FRONTEND_PORT'];
    delete process.env['PROMPTSHEON_CORS_ORIGIN'];
    process.env['PROMPTSHEON_FRONTEND_PORT'] = '3300';
    try {
      expect(loadConfig().server.corsOrigin).toBe('http://localhost:3300');
    } finally {
      if (previousOrigin === undefined) delete process.env['PROMPTSHEON_CORS_ORIGIN'];
      else process.env['PROMPTSHEON_CORS_ORIGIN'] = previousOrigin;
      if (previousFrontendPort === undefined) delete process.env['PROMPTSHEON_FRONTEND_PORT'];
      else process.env['PROMPTSHEON_FRONTEND_PORT'] = previousFrontendPort;
    }
  });
});
