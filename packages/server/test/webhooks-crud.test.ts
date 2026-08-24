import { describe, it, expect, beforeEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import Database from 'better-sqlite3';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { applyMigrations } from '@promptsheon/shared';
import {
  registerWebhookCrudRoutes,
  WebhookCrudStore,
  type OutgoingWebhook,
} from '../src/routes/webhooks-crud.js';
import { AuditChain } from '../src/audit/chain.js';

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

const ORG_A = '00000000-0000-4000-8000-000000000001';
const ORG_B = '00000000-0000-4000-8000-000000000002';

function openDb(): Database.Database {
  const db = new Database(':memory:');
  db.pragma('foreign_keys = ON');
  applyMigrations(db, loadAllMigrations());
  return db;
}

function buildApp(sharedStore?: WebhookCrudStore): {
  app: FastifyInstance;
  db: Database.Database;
  auditChain: AuditChain;
} {
  const app = Fastify();
  const db = openDb();
  const auditChain = new AuditChain(db);
  app.addHook('preHandler', (request, _reply, done) => {
    (request as Record<string, unknown>)['userId'] = 'u-test';
    (request as Record<string, unknown>)['orgContext'] = { organizationId: ORG_A, role: 'admin' };
    done();
  });
  registerWebhookCrudRoutes(app, { auditChain, store: sharedStore });
  return { app, db, auditChain };
}

describe('POST /api/webhooks', () => {
  let ctx: ReturnType<typeof buildApp>;
  beforeEach(async () => {
    ctx = buildApp();
    await ctx.app.ready();
  });

  it('rejects missing url with 422', async () => {
    const res = await ctx.app.inject({
      method: 'POST',
      url: '/api/webhooks',
      payload: { organizationId: ORG_A, label: 'gh', events: ['push'] },
    });
    expect(res.statusCode).toBe(422);
  });

  it('rejects non-url url with 422', async () => {
    const res = await ctx.app.inject({
      method: 'POST',
      url: '/api/webhooks',
      payload: { organizationId: ORG_A, label: 'gh', url: 'ftp://x', events: ['push'] },
    });
    expect(res.statusCode).toBe(422);
  });

  it('creates with defaults active=true and an id, then lists', async () => {
    const created = await ctx.app.inject({
      method: 'POST',
      url: '/api/webhooks',
      payload: {
        organizationId: ORG_A,
        label: 'gh',
        url: 'https://example.com/hook',
        events: ['push', 'pull_request'],
      },
    });
    expect(created.statusCode).toBe(201);
    const body: OutgoingWebhook = created.json();
    expect(body.id).toMatch(/^[0-9a-f-]{36}$/);
    expect(body.active).toBe(true);

    const list = await ctx.app.inject({ method: 'GET', url: '/api/webhooks' });
    expect(list.statusCode).toBe(200);
    const listBody = list.json() as { webhooks: OutgoingWebhook[] };
    expect(listBody.webhooks).toHaveLength(1);
    expect(listBody.webhooks[0]!.id).toBe(body.id);
  });
});

describe('PUT /api/webhooks/:id', () => {
  it('updates url + active and 404s on missing', async () => {
    const ctx = buildApp();
    await ctx.app.ready();
    const create = await ctx.app.inject({
      method: 'POST',
      url: '/api/webhooks',
      payload: {
        organizationId: ORG_A,
        label: 'gh',
        url: 'https://a.example.com',
        events: ['push'],
      },
    });
    const id = (create.json() as OutgoingWebhook).id;

    const update = await ctx.app.inject({
      method: 'PUT',
      url: `/api/webhooks/${id}`,
      payload: { url: 'https://b.example.com', active: false },
    });
    expect(update.statusCode).toBe(200);
    const updated = update.json() as OutgoingWebhook;
    expect(updated.url).toBe('https://b.example.com');
    expect(updated.active).toBe(false);

    const missing = await ctx.app.inject({
      method: 'PUT',
      url: '/api/webhooks/00000000-0000-4000-8000-000000000099',
      payload: { active: true },
    });
    expect(missing.statusCode).toBe(404);
  });
});

describe('DELETE /api/webhooks/:id', () => {
  it('removes and 404s the second time', async () => {
    const ctx = buildApp();
    await ctx.app.ready();
    const create = await ctx.app.inject({
      method: 'POST',
      url: '/api/webhooks',
      payload: {
        organizationId: ORG_A,
        label: 'gh',
        url: 'https://a.example.com',
        events: ['push'],
      },
    });
    const id = (create.json() as OutgoingWebhook).id;
    const del = await ctx.app.inject({ method: 'DELETE', url: `/api/webhooks/${id}` });
    expect(del.statusCode).toBe(204);
    const again = await ctx.app.inject({ method: 'DELETE', url: `/api/webhooks/${id}` });
    expect(again.statusCode).toBe(404);
  });
});

describe('org scoping', () => {
  it('does not return another org webhooks', async () => {
    const sharedStore = new WebhookCrudStore();
    const appA = Fastify();
    appA.addHook('preHandler', (req, _r, d) => {
      (req as Record<string, unknown>)['userId'] = 'uA';
      (req as Record<string, unknown>)['orgContext'] = { organizationId: ORG_A };
      d();
    });
    registerWebhookCrudRoutes(appA, {
      auditChain: new AuditChain(openDb()),
      store: sharedStore,
    });
    await appA.ready();
    await appA.inject({
      method: 'POST',
      url: '/api/webhooks',
      payload: {
        organizationId: ORG_A,
        label: 'A',
        url: 'https://a.example.com',
        events: ['push'],
      },
    });

    const appB = Fastify();
    appB.addHook('preHandler', (req, _r, d) => {
      (req as Record<string, unknown>)['userId'] = 'uB';
      (req as Record<string, unknown>)['orgContext'] = { organizationId: ORG_B };
      d();
    });
    registerWebhookCrudRoutes(appB, {
      auditChain: new AuditChain(openDb()),
      store: sharedStore,
    });
    await appB.ready();
    const listB = await appB.inject({ method: 'GET', url: '/api/webhooks' });
    expect((listB.json() as { webhooks: unknown[] }).webhooks).toHaveLength(0);
  });
});
