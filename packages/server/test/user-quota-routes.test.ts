import { describe, expect, it, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import Database from 'better-sqlite3';
import { applyMigrations } from '@promptsheon/shared';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { UserQuotaRepo } from '../src/repos/user-quota.js';
import { registerUserQuotaRoutes } from '../src/routes/user-quotas.js';

const directory = dirname(fileURLToPath(import.meta.url));
const migrations = join(directory, '..', '..', 'shared', 'db', 'migrations');

function openDb(): Database.Database {
  const db = new Database(':memory:');
  db.pragma('foreign_keys = ON');
  applyMigrations(db, readdirSync(migrations).filter((file) => file.endsWith('.up.sql')).map((file) => ({ version: Number(file.slice(0, 3)), name: file, up: readFileSync(join(migrations, file), 'utf8') })).sort((a, b) => a.version - b.version));
  return db;
}

describe('user quota routes', () => {
  let app: FastifyInstance;
  let db: Database.Database;

  beforeEach(async () => {
    db = openDb();
    app = Fastify();
    app.addHook('onRequest', async (request) => {
      const orgId = request.headers['x-test-org'];
      if (typeof orgId === 'string') request.orgContext = { userId: 'admin', orgId, role: 'admin' };
    });
    registerUserQuotaRoutes(app, { quotaRepo: new UserQuotaRepo(db) });
    await app.ready();
  });

  afterEach(async () => { await app.close(); db.close(); });

  it('creates, lists, updates, and deletes a quota without crossing org scope', async () => {
    const created = await app.inject({ method: 'POST', url: '/api/admin/user-quotas', payload: { organizationId: 'org-a', userId: 'user-a', label: 'Daily research', dailyRuns: 5 } });
    expect(created.statusCode).toBe(201);
    const id = (created.json() as { id: string }).id;

    const listed = await app.inject({ method: 'GET', url: '/api/admin/user-quotas?organizationId=org-a' });
    expect(listed.statusCode).toBe(200);
    expect(listed.json().items[0]).toMatchObject({ userId: 'user-a', dailyRuns: 5, enabled: true });

    const updated = await app.inject({ method: 'PATCH', url: `/api/admin/user-quotas/${id}`, payload: { enabled: false } });
    expect(updated.statusCode).toBe(200);
    expect(updated.json().enabled).toBe(false);

    const crossed = await app.inject({ method: 'GET', url: '/api/admin/user-quotas?organizationId=org-a', headers: { 'x-test-org': 'org-b' } });
    expect(crossed.statusCode).toBe(404);

    const removed = await app.inject({ method: 'DELETE', url: `/api/admin/user-quotas/${id}` });
    expect(removed.statusCode).toBe(204);
  });

  it('rejects duplicate policies for the same user', async () => {
    const payload = { organizationId: 'org-a', userId: 'user-a', label: 'First', dailyRuns: 1 };
    expect((await app.inject({ method: 'POST', url: '/api/admin/user-quotas', payload })).statusCode).toBe(201);
    const duplicate = await app.inject({ method: 'POST', url: '/api/admin/user-quotas', payload: { ...payload, label: 'Second' } });
    expect(duplicate.statusCode).toBe(409);
  });
});
