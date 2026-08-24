import { describe, it, expect, beforeEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import Database from 'better-sqlite3';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { applyMigrations } from '@promptsheon/shared';
import { FeatureFlagRepo } from '../src/repos/feature-flag.js';
import { registerFeatureFlagRoutes } from '../src/routes/feature-flag.js';
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

function openDb(): Database.Database {
  const db = new Database(':memory:');
  db.pragma('foreign_keys = ON');
  applyMigrations(db, loadAllMigrations());
  return db;
}

describe('feature-flags routes', () => {
  let app: FastifyInstance;
  let repo: FeatureFlagRepo;
  let db: Database.Database;

  beforeEach(async () => {
    db = openDb();
    repo = new FeatureFlagRepo(db);
    app = Fastify({ logger: false });
    app.addHook('preHandler', (request, _reply, done) => {
      (request as Record<string, unknown>)['userId'] = 'u-test';
      (request as Record<string, unknown>)['orgContext'] = { organizationId: '00000000-0000-4000-8000-000000000001', role: 'admin' };
      done();
    });
    registerFeatureFlagRoutes(app, { repo, auditChain: new AuditChain(db) });
    await app.ready();
  });

  it('lists the seeded default flags', async () => {
    const list = await app.inject({ method: 'GET', url: '/api/feature-flags' });
    expect(list.statusCode).toBe(200);
    const body = list.json() as { flags: Array<{ name: string; enabled: boolean }> };
    expect(body.flags.length).toBeGreaterThan(0);
    expect(body.flags.map((f) => f.name)).toContain('webhook_secret_ciphertext_v2');
  });

  it('upserts a new flag with value as JSON, returns the row', async () => {
    const upsert = await app.inject({
      method: 'PUT',
      url: '/api/feature-flags/test_flag',
      payload: { enabled: true, value: { threshold: 0.7, on: true } },
    });
    expect(upsert.statusCode).toBe(200);
    const body = upsert.json() as { name: string; enabled: boolean; value: unknown };
    expect(body.name).toBe('test_flag');
    expect(body.enabled).toBe(true);
    expect(body.value).toEqual({ threshold: 0.7, on: true });
  });

  it('rejects an invalid name with 422', async () => {
    const bad = await app.inject({
      method: 'PUT',
      url: '/api/feature-flags/HasSpace',
      payload: { enabled: false },
    });
    expect(bad.statusCode).toBe(422);
  });

  it('updates an existing flag without losing description', async () => {
    repo.upsert({ name: 'stable', enabled: false, description: 'original' });
    const upd = await app.inject({
      method: 'PUT',
      url: '/api/feature-flags/stable',
      payload: { enabled: true },
    });
    expect(upd.statusCode).toBe(200);
    const body = upd.json() as { name: string; enabled: boolean; description: string };
    expect(body.description).toBe('original');
    expect(body.enabled).toBe(true);
  });

  it('deletes a flag', async () => {
    repo.upsert({ name: 'byebye', enabled: false });
    const del = await app.inject({ method: 'DELETE', url: '/api/feature-flags/byebye' });
    expect(del.statusCode).toBe(204);
    expect(repo.find('byebye')).toBeNull();
  });

  it('returns 404 on delete of a missing flag', async () => {
    const del = await app.inject({ method: 'DELETE', url: '/api/feature-flags/nope' });
    expect(del.statusCode).toBe(404);
  });

  it('round-trips value as JSON across upserts', async () => {
    await app.inject({
      method: 'PUT',
      url: '/api/feature-flags/round',
      payload: { enabled: false, value: [1, 2, 3] },
    });
    const reread = repo.find('round');
    expect(reread?.value).toEqual([1, 2, 3]);
  });
});
