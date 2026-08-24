import { describe, it, expect } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import Database from 'better-sqlite3';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { applyMigrations } from '@promptsheon/shared';
import { registerAuditRoutes } from '../src/routes/audit.js';
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

describe('audit/verify whitelist', () => {
  it('does not require auth headers (passes the auth middleware bypass)', async () => {
    const db = openDb();
    const auditChain = new AuditChain(db);
    auditChain.append({
      userId: 'system',
      action: 'test.append',
      resource: 'test',
      details: '{}',
      resourceKind: 'test',
      resourceId: 't1',
    });
    const app = Fastify({ logger: false });
    registerAuditRoutes(app, { auditChain, db });
    await app.ready();
    const res = await app.inject({ method: 'GET', url: '/api/audit/verify' });
    expect(res.statusCode).toBe(200);
    const body = res.json() as { valid?: boolean };
    expect(body.valid).toBe(true);
  });

  it('returns valid:true for an empty chain', async () => {
    const db = openDb();
    const auditChain = new AuditChain(db);
    const app = Fastify({ logger: false });
    registerAuditRoutes(app, { auditChain, db });
    await app.ready();
    const res = await app.inject({ method: 'GET', url: '/api/audit/verify' });
    expect(res.statusCode).toBe(200);
    const body = res.json() as { valid?: boolean };
    expect(body.valid).toBe(true);
  });

  it('exposes chain state unauthenticated', async () => {
    const db = openDb();
    const auditChain = new AuditChain(db);
    const app = Fastify({ logger: false });
    registerAuditRoutes(app, { auditChain, db });
    await app.ready();
    const res = await app.inject({ method: 'GET', url: '/api/audit/state' });
    expect(res.statusCode).toBe(200);
  });
});
