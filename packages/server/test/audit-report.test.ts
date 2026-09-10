import { describe, it, expect } from 'vitest';
import Database from 'better-sqlite3';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { applyMigrations } from '@promptsheon/shared';
import Fastify, { type FastifyInstance } from 'fastify';
import { AuditChain } from '../src/audit/chain.js';
import { registerAuditReportRoutes } from '../src/routes/audit-report.js';

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

function buildApp(): { app: FastifyInstance; db: Database.Database } {
  const db = openDb();
  db.prepare(
    `INSERT INTO orgs (id,name,slug,created_at,updated_at) VALUES ('org-1','O','o',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
  ).run();
  db.prepare(
    `INSERT INTO users (id, email, name, role, created_at, updated_at)
     VALUES ('u-alice','alice@e.test','Alice','admin',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
  ).run();
  db.prepare(
    `INSERT INTO users (id, email, name, role, created_at, updated_at)
     VALUES ('u-bob','bob@e.test','Bob','admin',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
  ).run();
  const audit = new AuditChain(db);
  audit.append({
    userId: 'u-alice',
    action: 'workspace.create',
    resource: 'workspace',
    details: '{"id":"w-1"}',
    resourceKind: 'workspace',
    resourceId: 'w-1',
  });
  audit.append({
    userId: 'u-bob',
    action: 'release.activate',
    resource: 'release',
    details: '{"id":"r-1"}',
    resourceKind: 'release',
    resourceId: 'r-1',
  });
  const app = Fastify({ logger: false });
  app.addHook('preHandler', (request, _reply, done) => {
    (request as Record<string, unknown>)['userId'] = 'u-test';
    (request as Record<string, unknown>)['orgContext'] = { organizationId: 'org-1' };
    done();
  });
  registerAuditReportRoutes(app, { auditChain: audit });
  return { app, db };
}

describe('GET /api/audit/report', () => {
  it('returns a signed JSON report with the chain head + entry rows', async () => {
    const { app } = buildApp();
    const r = await app.inject({
      method: 'GET',
      url: '/api/audit/report',
      headers: { 'x-user-id': 'u-test', 'x-org-id': 'org-1' },
    });
    expect(r.statusCode).toBe(200);
    const body = r.json() as {
      organizationId: string;
      entryCount: number;
      chainValid: boolean;
      chainHead: string;
      entries: Array<{ actor: string; action: string }>;
      signature: { algorithm: string; value: string };
    };
    expect(body.organizationId).toBe('org-1');
    expect(body.entryCount).toBe(2);
    expect(body.chainValid).toBe(true);
    expect(body.chainHead).toMatch(/^[0-9a-f]{64}$/);
    expect(body.entries.map((e) => e.actor)).toEqual(['u-alice', 'u-bob']);
    expect(body.signature.value).toMatch(/^[0-9a-f]{64}$/);
  });

  it('rejects requests without an org context', async () => {
    const db = openDb();
    const audit = new AuditChain(db);
    const app = Fastify({ logger: false });
    registerAuditReportRoutes(app, { auditChain: audit });
    const r = await app.inject({
      method: 'GET',
      url: '/api/audit/report',
      headers: { 'x-user-id': 'u-test' },
    });
    expect(r.statusCode).toBe(401);
  });

  it('filters by actor', async () => {
    const { app } = buildApp();
    const r = await app.inject({
      method: 'GET',
      url: '/api/audit/report?actor=u-alice',
      headers: { 'x-user-id': 'u-test', 'x-org-id': 'org-1' },
    });
    const body = r.json() as { entries: unknown[]; entryCount: number };
    expect(body.entryCount).toBe(1);
  });

  it('flags broken chain correctly when an entry was tampered', async () => {
    // Build a DB, drop the append-only trigger, then append one
    // entry via AuditChain. This simulates a tampering scenario.
    const db = openDb();
    db.exec('DROP TRIGGER IF EXISTS audit_entries_no_update');
    db.prepare(
      `INSERT INTO orgs (id,name,slug,created_at,updated_at) VALUES ('org-1','O','o',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
    ).run();
    const audit = new AuditChain(db);
    audit.append({
      userId: 'u',
      action: 'a',
      resource: 'r',
      details: '{}',
      resourceKind: 'r',
      resourceId: '1',
    });
    // Tamper: bypass the now-dropped trigger and update directly.
    db.prepare(
      `UPDATE audit_entries SET details = '{"tampered":true}' WHERE id IN (SELECT id FROM audit_entries LIMIT 1)`,
    ).run();
    const app = Fastify({ logger: false });
    app.addHook('preHandler', (request, _reply, done) => {
      (request as Record<string, unknown>)['orgContext'] = { organizationId: 'org-1' };
      done();
    });
    registerAuditReportRoutes(app, { auditChain: audit });
    const r = await app.inject({
      method: 'GET',
      url: '/api/audit/report',
      headers: { 'x-org-id': 'org-1' },
    });
    const body = r.json() as { chainValid: boolean };
    expect(body.chainValid).toBe(false);
  });

  it('sends Content-Disposition attachment header', async () => {
    const { app } = buildApp();
    const r = await app.inject({
      method: 'GET',
      url: '/api/audit/report',
      headers: { 'x-user-id': 'u-test', 'x-org-id': 'org-1' },
    });
    const cd = r.headers['content-disposition'];
    expect(cd).toMatch(/^attachment; filename=/);
  });
});
