import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import { registerAuditRoutes } from '../src/routes/audit.js';
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

describe('audit routes', () => {
  let db: ReturnType<typeof import('better-sqlite3')>;
  let app: FastifyInstance;
  let audit: AuditChain;

  beforeEach(async () => {
    db = (await import('better-sqlite3')).default(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
    audit = new AuditChain(db);
    app = Fastify({ logger: false });
    app.setErrorHandler((error, _request, reply) => {
      if (error.statusCode) return reply.code(error.statusCode).send({ error: { code: 'APP_ERROR', message: error.message } });
      return reply.code(500).send({ error: { code: 'INTERNAL_ERROR', message: error.message } });
    });
    await app.register(async (instance) => {
      registerAuditRoutes(instance, { auditChain: audit, db });
    });
    await app.ready();
  });

  afterEach(async () => {
    await app.close();
    db.close();
  });

  it('appends + lists entries, returns verify state', async () => {
    audit.append({ userId: 'api', action: 'test.action', resource: 'thing', details: '{}', resourceKind: 'thing', resourceId: '1' });
    audit.append({ userId: 'api', action: 'test.action2', resource: 'thing', details: '{}', resourceKind: 'thing', resourceId: '1' });

    const list = await app.inject({ method: 'GET', url: '/api/audit' });
    const listBody = list.json() as { entries: Array<{ action: string }> };
    expect(listBody.entries.length).toBe(2);

    const verify = await app.inject({ method: 'GET', url: '/api/audit/verify' });
    expect(verify.statusCode).toBe(200);
    const verifyBody = verify.json() as { valid: boolean };
    expect(verifyBody.valid).toBe(true);

    const state = await app.inject({ method: 'GET', url: '/api/audit/state' });
    const stateBody = state.json() as { lastHash: string; lastRowid: number };
    expect(stateBody.lastHash).toMatch(/^[0-9a-f]{64}$/);
    expect(stateBody.lastRowid).toBeGreaterThan(0);
  });

  it('filters entries by action', async () => {
    audit.append({ userId: 'api', action: 'alpha', resource: 'r', details: '{}', resourceKind: 'r', resourceId: '1' });
    audit.append({ userId: 'api', action: 'beta', resource: 'r', details: '{}', resourceKind: 'r', resourceId: '1' });
    const res = await app.inject({ method: 'GET', url: '/api/audit?action=alpha' });
    const body = res.json() as { entries: Array<{ action: string }> };
    expect(body.entries.length).toBe(1);
    expect(body.entries[0]!.action).toBe('alpha');
  });
});