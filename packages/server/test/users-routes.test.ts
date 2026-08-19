import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import { registerUserRoutes } from '../src/routes/users.js';
import { UserRepo } from '../src/repos/user.js';
import { AuditChain } from '../src/audit/chain.js';
import { applyMigrations } from '@promptsheon/shared';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

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

describe('users routes', () => {
  let db: ReturnType<typeof import('better-sqlite3')>;
  let app: FastifyInstance;
  let userRepo: UserRepo;
  let audit: AuditChain;

  beforeEach(async () => {
    db = (await import('better-sqlite3')).default(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
    userRepo = new UserRepo(db);
    audit = new AuditChain(db);
    app = Fastify({ logger: false });
    app.setErrorHandler((error, _request, reply) => {
      if (error.statusCode) return reply.code(error.statusCode).send({ error: { code: 'APP_ERROR', message: error.message } });
      return reply.code(500).send({ error: { code: 'INTERNAL_ERROR', message: error.message } });
    });
    await app.register(async (instance) => {
      registerUserRoutes(instance, { userRepo, auditChain: audit });
    });
    await app.ready();
  });

  afterEach(async () => {
    await app.close();
    db.close();
  });

  it('creates and lists a user', async () => {
    const create = await app.inject({ method: 'POST', url: '/api/users', payload: { email: 'a@b.com', name: 'Alice' } });
    expect(create.statusCode).toBe(201);
    const list = await app.inject({ method: 'GET', url: '/api/users' });
    expect(list.statusCode).toBe(200);
    const body = list.json() as { users: Array<{ id: string; email: string; name: string }> };
    expect(body.users.some((u) => u.email === 'a@b.com' && u.name === 'Alice')).toBe(true);
  });

  it('updates a user role and emits an audit entry', async () => {
    const create = await app.inject({ method: 'POST', url: '/api/users', payload: { email: 'b@c.com', name: 'Bob' } });
    const created = create.json() as { id: string };
    const update = await app.inject({ method: 'PUT', url: `/api/users/${created.id}/role`, payload: { role: 'admin' } });
    expect(update.statusCode).toBe(200);
    const verify = audit.verify();
    expect(verify.valid).toBe(true);
  });

  it('returns 404 for unknown user on role update', async () => {
    const r = await app.inject({ method: 'PUT', url: '/api/users/nope/role', payload: { role: 'admin' } });
    expect(r.statusCode).toBe(404);
  });
});