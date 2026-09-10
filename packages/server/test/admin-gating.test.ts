import { describe, it, expect, beforeEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import Database from 'better-sqlite3';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { applyMigrations } from '@promptsheon/shared';
import { registerApiKeyRoutes } from '../src/routes/api-keys.js';
import { registerUserRoutes } from '../src/routes/users.js';
import { registerSettingsRoutes } from '../src/routes/settings.js';
import { registerFeatureFlagRoutes } from '../src/routes/feature-flag.js';
import { registerWebhookCrudRoutes } from '../src/routes/webhooks-crud.js';
import { ApiKeyRepo } from '../src/repos/api-key.js';
import { UserRepo } from '../src/repos/user.js';
import { FeatureFlagRepo } from '../src/repos/feature-flag.js';
import { AuditChain } from '../src/audit/chain.js';
import { SettingsResolver } from '../src/settings/resolver.js';
import { SystemConfigRepo } from '../src/repos/system-config.js';

const __dirname = dirname(fileURLToPath(import.meta.url));
const MIGRATIONS_DIR = join(__dirname, '..', '..', 'shared', 'db', 'migrations');

function loadAllMigrations() {
  return readdirSync(MIGRATIONS_DIR)
    .filter((f) => f.endsWith('.up.sql'))
    .map((f) => {
      const version = parseInt(f.split('_')[0], 10);
      const up = readFileSync(join(MIGRATIONS_DIR, f), 'utf-8');
      return { version, name: f, up };
    })
    .filter((m) => m.version !== 0)
    .sort((a, b) => a.version - b.version);
}

function openDb(): Database.Database {
  const db = new Database(':memory:');
  db.pragma('foreign_keys = ON');
  applyMigrations(db, loadAllMigrations());
  return db;
}

function buildApp(role: 'admin' | 'editor' | 'reader'): { app: FastifyInstance; db: Database.Database } {
  const db = openDb();
  const app = Fastify({ logger: false });
  app.setErrorHandler((error, _request, reply) => {
    if (error.statusCode) return reply.code(error.statusCode).send({ error: { code: 'APP_ERROR', message: error.message } });
    return reply.code(500).send({ error: { code: 'INTERNAL', message: error.message } });
  });
  app.addHook('preHandler', (request, _reply, done) => {
    (request as Record<string, unknown>)['userId'] = 'u-test';
    (request as Record<string, unknown>)['orgContext'] = {
      organizationId: '00000000-0000-4000-8000-000000000001',
      role,
    };
    done();
  });
  const audit = new AuditChain(db);
  registerApiKeyRoutes(app, { apiKeyRepo: new ApiKeyRepo(db), auditChain: audit });
  registerUserRoutes(app, { userRepo: new UserRepo(db), auditChain: audit });
  registerSettingsRoutes(app, new SettingsResolver({}, {}, new SystemConfigRepo(db)));
  registerFeatureFlagRoutes(app, { repo: new FeatureFlagRepo(db), auditChain: audit });
  registerWebhookCrudRoutes(app, { auditChain: audit });
  return { app, db };
}

describe('admin gating', () => {
  describe('admin role', () => {
    let ctx: ReturnType<typeof buildApp>;
    beforeEach(async () => {
      ctx = buildApp('admin');
      await ctx.app.ready();
    });

    it('200 on GET /api/users', async () => {
      const r = await ctx.app.inject({ method: 'GET', url: '/api/users' });
      expect(r.statusCode).toBe(200);
    });

    it('201 on POST /api/users', async () => {
      const r = await ctx.app.inject({
        method: 'POST',
        url: '/api/users',
        payload: { email: 'a@b.test', name: 'A' },
      });
      expect(r.statusCode).toBe(201);
    });

    it('200 on PUT /api/settings/foo', async () => {
      const r = await ctx.app.inject({
        method: 'PUT',
        url: '/api/settings/foo',
        payload: { value: 'bar' },
      });
      expect(r.statusCode).toBe(200);
    });
  });

  describe('reader role', () => {
    let ctx: ReturnType<typeof buildApp>;
    beforeEach(async () => {
      ctx = buildApp('reader');
      await ctx.app.ready();
    });

    it('403 on GET /api/users', async () => {
      const r = await ctx.app.inject({ method: 'GET', url: '/api/users' });
      expect(r.statusCode).toBe(403);
    });

    it('403 on POST /api/users', async () => {
      const r = await ctx.app.inject({
        method: 'POST',
        url: '/api/users',
        payload: { email: 'a@b.test', name: 'A' },
      });
      expect(r.statusCode).toBe(403);
    });

    it('403 on PUT /api/settings/foo', async () => {
      const r = await ctx.app.inject({
        method: 'PUT',
        url: '/api/settings/foo',
        payload: { value: 'bar' },
      });
      expect(r.statusCode).toBe(403);
    });

    it('403 on GET /api/feature-flags', async () => {
      const r = await ctx.app.inject({ method: 'GET', url: '/api/feature-flags' });
      expect(r.statusCode).toBe(403);
    });

    it('403 on GET /api/webhooks', async () => {
      const r = await ctx.app.inject({ method: 'GET', url: '/api/webhooks' });
      expect(r.statusCode).toBe(403);
    });
  });
});
