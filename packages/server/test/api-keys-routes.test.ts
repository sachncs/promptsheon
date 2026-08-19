import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import { registerApiKeyRoutes } from '../src/routes/api-keys.js';
import { ApiKeyRepo } from '../src/repos/api-key.js';
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

describe('api-keys routes', () => {
  let db: ReturnType<typeof import('better-sqlite3')>;
  let app: FastifyInstance;
  let repo: ApiKeyRepo;
  let audit: AuditChain;

  beforeEach(async () => {
    db = (await import('better-sqlite3')).default(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
    repo = new ApiKeyRepo(db);
    audit = new AuditChain(db);
    app = Fastify({ logger: false });
    app.setErrorHandler((error, _request, reply) => {
      if (error.statusCode) return reply.code(error.statusCode).send({ error: { code: 'APP_ERROR', message: error.message } });
      return reply.code(500).send({ error: { code: 'INTERNAL_ERROR', message: error.message } });
    });
    await app.register(async (instance) => {
      registerApiKeyRoutes(instance, { apiKeyRepo: repo, auditChain: audit });
    });
    await app.ready();
  });

  afterEach(async () => {
    await app.close();
    db.close();
  });

  it('creates an API key, returns plaintext once, then lists with prefix only', async () => {
    const create = await app.inject({ method: 'POST', url: '/api/api-keys', payload: { name: 'k1', userId: 'api', role: 'reader' } });
    expect(create.statusCode).toBe(201);
    const body = create.json() as { key: string; keyPrefix: string; id: string };
    expect(body.key).toMatch(/^pk_/);
    expect(body.keyPrefix).toBe(body.key.slice(0, 12));

    const list = await app.inject({ method: 'GET', url: '/api/api-keys' });
    const listBody = list.json() as { keys: Array<{ id: string; key: unknown }> };
    expect(listBody.keys[0]?.id).toBe(body.id);
    expect((listBody.keys[0] as { key: unknown }).key).toBeUndefined();
  });

  it('revokes an API key and writes an audit entry', async () => {
    const create = await app.inject({ method: 'POST', url: '/api/api-keys', payload: { name: 'k2', userId: 'api', role: 'reader' } });
    const { id } = create.json() as { id: string };
    const del = await app.inject({ method: 'DELETE', url: `/api/api-keys/${id}` });
    expect(del.statusCode).toBe(204);
    expect(audit.verify().valid).toBe(true);
  });

  it('returns 404 when revoking a missing key', async () => {
    const r = await app.inject({ method: 'DELETE', url: '/api/api-keys/missing' });
    expect(r.statusCode).toBe(404);
  });
});