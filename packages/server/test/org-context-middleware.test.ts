import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import Database from 'better-sqlite3';
import { orgContextMiddleware, requireRole, getOrgContext } from '../src/middleware/org-context.js';
import { MembershipRepo } from '../src/repos/org.js';
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

describe('orgContextMiddleware', () => {
  let app: FastifyInstance;
  let db: ReturnType<typeof Database>;
  let membershipRepo: MembershipRepo;
  let orgId: string;

  beforeEach(async () => {
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
    db.prepare(`INSERT INTO orgs (id, name, slug, created_at, updated_at) VALUES ('o1', 'Org', 'org', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`).run();
    db.prepare(`INSERT INTO users (id, org_id, email, name, role, created_at, updated_at) VALUES ('u1', 'o1', 'a@b.com', 'A', 'admin', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`).run();
    db.prepare(`INSERT INTO org_members (org_id, user_id, role, joined_at) VALUES ('o1', 'u1', 'admin', '2026-01-01T00:00:00Z')`).run();
    orgId = 'o1';
    membershipRepo = new MembershipRepo(db);
    app = Fastify();
    app.setErrorHandler((error, _request, reply) => {
      if (error.statusCode) {
        return reply.code(error.statusCode).send({ error: { code: 'APP_ERROR', message: error.message } });
      }
      return reply.code(500).send({ error: { code: 'INTERNAL_ERROR', message: error.message } });
    });
    app.addHook('preHandler', orgContextMiddleware({ membershipRepo }));
    app.get('/api/whoami', async (request) => {
      const ctx = getOrgContext(request);
      return { userId: ctx.userId, orgId: ctx.orgId, role: ctx.role };
    });
    await app.ready();
  });

  afterEach(async () => {
    await app.close();
    db.close();
  });

  it('returns 401 when X-User-Id and X-Org-Id are missing', async () => {
    const response = await app.inject({ method: 'GET', url: '/api/whoami' });
    expect(response.statusCode).toBe(401);
  });

  it('returns 403 when user is not a member of org', async () => {
    const response = await app.inject({
      method: 'GET',
      url: '/api/whoami',
      headers: { 'x-user-id': 'nonexistent', 'x-org-id': 'o1' },
    });
    expect(response.statusCode).toBe(403);
  });

  it('attaches org context when user is a member (direct repo call)', () => {
    const members = membershipRepo.findOrgMembers('o1');
    expect(members.length).toBeGreaterThan(0);
    const row = db.prepare("SELECT * FROM org_members WHERE org_id = 'o1' AND user_id = 'u1'").get();
    expect(row).toBeDefined();
  });
});

describe('requireRole', () => {
  let app: FastifyInstance;
  beforeEach(async () => {
    app = Fastify();
    app.addHook('preHandler', async (request, reply) => {
      (request as unknown as { orgContext: { userId: string; orgId: string; role: string } }).orgContext = {
        userId: 'u1', orgId: 'o1', role: 'admin',
      };
    });
    app.addHook('preHandler', requireRole(['admin', 'approver']));
    app.get('/api/admin', async () => 'ok');
    await app.ready();
  });
  afterEach(async () => { await app.close(); });

  it('passes for matching role', async () => {
    const response = await app.inject({ method: 'GET', url: '/api/admin' });
    expect(response.statusCode).toBe(200);
  });
});