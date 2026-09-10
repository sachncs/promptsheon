import { describe, it, expect, beforeEach } from 'vitest';
import type { FastifyRequest, FastifyReply } from 'fastify';
import { authMiddleware } from '../src/middleware/auth.js';
import type { AppConfig } from '@promptsheon/shared';

type HeaderMap = Record<string, string | string[] | undefined>;

function makeReq(headers: HeaderMap, url = '/api/workspaces'): FastifyRequest {
  return {
    url,
    headers: headers as unknown as FastifyRequest['headers'],
  } as unknown as FastifyRequest;
}

function makeReply(): { reply: FastifyReply; code: number; body: unknown } {
  const state = { code: 200, body: undefined as unknown };
  const reply = {
    code(c: number) {
      state.code = c;
      return this;
    },
    send(b: unknown) {
      state.body = b;
      return this;
    },
  } as unknown as FastifyReply;
  return { reply, get code() { return state.code; }, get body() { return state.body; } } as never;
}

function makeApiKeyRepo(opts: {
  find?: (keyHash: string) => Promise<{ id: string; userId: string; role: string; revoked: boolean; expiresAt?: string } | null>;
  updateLastUsed?: (id: string) => Promise<void>;
}) {
  return {
    findByKeyHash: opts.find ?? (async () => null),
    updateLastUsed: opts.updateLastUsed ?? (async () => undefined),
  };
}

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
  auth: {
    enabled: true,
    jwtSecret: 'test-secret',
  },
  selfEvolve: {
    enabled: false,
    defaultCooldownSec: 900,
    maxConcurrent: 3,
  },
};

describe('authMiddleware (issue #45 — X-User-Id bypass fix)', () => {
  it('rejects 401 when auth is enabled and only X-User-Id is sent', async () => {
    const apiKeyRepo = makeApiKeyRepo({});
    const mw = authMiddleware(baseConfig, apiKeyRepo);
    const req = makeReq({
      'x-user-id': 'admin',
      'x-org-id': 'o1',
    });
    const reply = makeReply();
    await mw(req, reply as never);
    expect(reply.code).toBe(401);
    expect(reply.body).toEqual({
      error: { code: 'UNAUTHORIZED', message: 'Missing authorization header' },
    });
  });

  it('rejects 401 when auth is enabled and there is no Authorization header at all', async () => {
    const apiKeyRepo = makeApiKeyRepo({});
    const mw = authMiddleware(baseConfig, apiKeyRepo);
    const req = makeReq({});
    const reply = makeReply();
    await mw(req, reply as never);
    expect(reply.code).toBe(401);
  });

  it('accepts Bearer token and stamps userId + userRole on the request', async () => {
    const apiKeyRepo = makeApiKeyRepo({
      find: async () => ({
        id: 'k1',
        userId: 'u1',
        role: 'admin',
        revoked: false,
      }),
      updateLastUsed: async () => undefined,
    });
    const mw = authMiddleware(baseConfig, apiKeyRepo);
    const req = makeReq({ authorization: 'Bearer abc123' });
    const reply = makeReply();
    await mw(req, reply as never);
    expect(reply.code).toBe(200);
    expect((req as unknown as Record<string, string>).userId).toBe('u1');
    expect((req as unknown as Record<string, string>).userRole).toBe('admin');
  });

  it('rejects revoked Bearer tokens with 401', async () => {
    const apiKeyRepo = makeApiKeyRepo({
      find: async () => ({ id: 'k1', userId: 'u1', role: 'admin', revoked: true }),
    });
    const mw = authMiddleware(baseConfig, apiKeyRepo);
    const req = makeReq({ authorization: 'Bearer abc' });
    const reply = makeReply();
    await mw(req, reply as never);
    expect(reply.code).toBe(401);
  });

  it('rejects expired Bearer tokens with 401', async () => {
    const apiKeyRepo = makeApiKeyRepo({
      find: async () => ({
        id: 'k1',
        userId: 'u1',
        role: 'admin',
        revoked: false,
        expiresAt: '2020-01-01T00:00:00Z',
      }),
    });
    const mw = authMiddleware(baseConfig, apiKeyRepo);
    const req = makeReq({ authorization: 'Bearer abc' });
    const reply = makeReply();
    await mw(req, reply as never);
    expect(reply.code).toBe(401);
  });

  it('honours X-User-Id fallback only when auth is disabled', async () => {
    const config: AppConfig = {
      ...baseConfig,
      auth: { enabled: false, jwtSecret: '' },
    };
    const mw = authMiddleware(config, makeApiKeyRepo({}));
    const req = makeReq({ 'x-user-id': 'dev' });
    const reply = makeReply();
    await mw(req, reply as never);
    expect(reply.code).toBe(200);
    expect((req as unknown as Record<string, string>).userId).toBe('dev');
  });

  it('tags the request as bootstrap on /api/bootstrap/* paths', async () => {
    const mw = authMiddleware(baseConfig, makeApiKeyRepo({}));
    const req = makeReq({}, '/api/bootstrap/admin');
    const reply = makeReply();
    await mw(req, reply as never);
    expect(reply.code).toBe(200);
    expect((req as unknown as Record<string, string>).userId).toBe('bootstrap');
  });

  it('tags the request as public on the documented public paths', async () => {
    const mw = authMiddleware(baseConfig, makeApiKeyRepo({}));
    for (const path of ['/api/health', '/api/openapi.json', '/api/audit/verify', '/api/audit/state']) {
      const req = makeReq({}, path);
      const reply = makeReply();
      await mw(req, reply as never);
      expect(reply.code).toBe(200);
      expect((req as unknown as Record<string, string>).userId).toBe('public');
    }
  });
});
