import { describe, it, expect } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import Database from 'better-sqlite3';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { applyMigrations } from '@promptsheon/shared';
import { registerPreconditionRoutes } from '../src/routes/precondition.js';
import { PreconditionRepo } from '../src/repos/precondition.js';
import type { Precondition } from '@promptsheon/shared';

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

const ORG = '00000000-0000-4000-8000-000000000001';
const WS = '00000000-0000-4000-8000-00000000000a';
const PROJECT = '00000000-0000-4000-8000-00000000000b';
const CAP = '00000000-0000-4000-8000-00000000000c';

function seed(db: Database.Database) {
  db.prepare(`INSERT INTO orgs (id,name,slug,created_at,updated_at) VALUES (?, 'O','o',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).run(ORG);
  db.prepare(`INSERT INTO workspaces (id,name,organization,created_at,updated_at) VALUES (?, 'W','O',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).run(WS);
  db.prepare(`INSERT INTO projects (id,workspace_id,name,description,created_at,updated_at) VALUES (?, ?, 'P','', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).run(PROJECT, WS);
  db.prepare(`INSERT INTO capabilities (id,project_id,name,description,created_at,updated_at) VALUES (?, ?, 'C','', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).run(CAP, PROJECT);
}

describe('PUT /api/preconditions/:id', () => {
  it('flips enabled and returns the row', async () => {
    const db = openDb();
    seed(db);
    const repo = new PreconditionRepo(db);
    const created = repo.create({
      capabilityId: CAP,
      name: 'Linter',
      command: 'eslint .',
      enabled: true,
    });

    const app = Fastify({ logger: false });
    registerPreconditionRoutes(app, repo);
    await app.ready();

    const r = await app.inject({
      method: 'PUT',
      url: `/api/preconditions/${created.id}`,
      payload: { enabled: false, timeoutSec: 90 },
    });
    expect(r.statusCode).toBe(200);
    const body = r.json() as Precondition;
    expect(body.enabled).toBe(false);
    expect(body.timeoutSec).toBe(90);
    expect(body.updatedAt).toBeDefined();
  });

  it('updates name + command alone', async () => {
    const db = openDb();
    seed(db);
    const repo = new PreconditionRepo(db);
    const created = repo.create({
      capabilityId: CAP,
      name: 'Linter',
      command: 'eslint .',
      enabled: true,
    });
    const app = Fastify({ logger: false });
    app.setErrorHandler((err, _r, reply) => {
      process.stderr.write('CAPTURED ERROR: ' + ((err as Error).stack ?? (err as Error).message) + '\n');
      return reply.code(500).send({ error: { code: 'INTERNAL', message: (err as Error).message } });
    });
    registerPreconditionRoutes(app, repo);
    await app.ready();

    const r = await app.inject({
      method: 'PUT',
      url: `/api/preconditions/${created.id}`,
      payload: { command: 'eslint --quiet .' },
    });
    if (r.statusCode !== 200) {
      process.stderr.write('RESPONSE BODY: ' + r.body + '\n');
    }
    expect(r.statusCode).toBe(200);
    const body = r.json() as Precondition;
    expect(body.command).toBe('eslint --quiet .');
    expect(body.name).toBe('Linter');
  });

  it('rejects bad timeoutSec with 422', async () => {
    const db = openDb();
    seed(db);
    const repo = new PreconditionRepo(db);
    const created = repo.create({
      capabilityId: CAP,
      name: 'Linter',
      command: 'eslint .',
      enabled: true,
    });
    const app = Fastify({ logger: false });
    registerPreconditionRoutes(app, repo);
    await app.ready();
    const r = await app.inject({
      method: 'PUT',
      url: `/api/preconditions/${created.id}`,
      payload: { timeoutSec: 99999 },
    });
    expect(r.statusCode).toBe(422);
  });

  it('returns 404 on unknown id', async () => {
    const db = openDb();
    seed(db);
    const app = Fastify({ logger: false });
    registerPreconditionRoutes(app, new PreconditionRepo(db));
    await app.ready();
    const r = await app.inject({
      method: 'PUT',
      url: '/api/preconditions/unknown-id',
      payload: { enabled: false },
    });
    expect(r.statusCode).toBe(404);
  });
});
