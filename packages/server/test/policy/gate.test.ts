import { describe, it, expect, beforeAll, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import { resolve } from 'node:path';
import { registerOrgTeamRoutes } from '../../src/routes/org-team.js';
import { OrgRepo, TeamRepo, MembershipRepo } from '../../src/repos/org.js';
import { CedarAuthorizer, installDefaultAuthorizer } from '../../src/policy/gate.js';
import { applyMigrations } from '@promptsheon/shared';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const MIGRATIONS_DIR = join(__dirname, '..', '..', '..', 'shared', 'db', 'migrations');

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

describe('Cedar preHandler on /api/orgs', () => {
  let app: FastifyInstance;
  let db: ReturnType<typeof import('better-sqlite3')>;
  let orgRepo: OrgRepo;
  let teamRepo: TeamRepo;
  let membershipRepo: MembershipRepo;
  let authorizer: CedarAuthorizer;

  beforeAll(() => {
    authorizer = new CedarAuthorizer({
      policyPath: resolve(process.cwd(), 'policies/promptsheon.cedar'),
    });
    authorizer.load();
    installDefaultAuthorizer(authorizer);
  });

  beforeEach(async () => {
    const Database = (await import('better-sqlite3')).default;
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
    orgRepo = new OrgRepo(db);
    teamRepo = new TeamRepo(db);
    membershipRepo = new MembershipRepo(db);
    app = Fastify();
    app.setErrorHandler((error, _request, reply) => {
      if (error.name === 'NotFoundError') return reply.code(404).send({ error: { code: 'NOT_FOUND', message: error.message } });
      if (error.statusCode) return reply.code(error.statusCode).send({ error: { code: 'APP_ERROR', message: error.message } });
      return reply.code(500).send({ error: { code: 'INTERNAL_ERROR', message: error.message } });
    });
    await app.register(async (instance) => {
      await registerOrgTeamRoutes(instance, { orgRepo, teamRepo, membershipRepo });
    });
    await app.ready();
  });

  afterEach(async () => {
    await app.close();
    db.close();
  });

  it('admin principal passes the Cedar gate (POST /api/orgs → 201)', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/api/orgs',
      headers: {
        'x-principal-type': 'role',
        'x-principal-id': 'admin',
        'x-org-id': 'org-1',
      },
      payload: { name: 'Acme', slug: 'acme' },
    });
    expect(response.statusCode).toBe(201);
  });

  it('viewer principal is denied by the Cedar gate (POST /api/orgs → 403)', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/api/orgs',
      headers: {
        'x-principal-type': 'role',
        'x-principal-id': 'viewer',
        'x-org-id': 'org-1',
      },
      payload: { name: 'Acme', slug: 'acme' },
    });
    expect(response.statusCode).toBe(403);
    const body = response.json() as { error: { code: string } };
    expect(body.error.code).toBe('FORBIDDEN');
  });

  it('no principal + no system actor falls through (POST /api/orgs → 201, preserving old behaviour)', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/api/orgs',
      payload: { name: 'Acme', slug: 'acme' },
    });
    expect(response.statusCode).toBe(201);
  });

  it('System principal (override on) passes the gate', async () => {
    const prev = process.env['PROMPTSHEON_ALLOW_SYSTEM_ACTOR'];
    process.env['PROMPTSHEON_ALLOW_SYSTEM_ACTOR'] = 'true';
    try {
      const response = await app.inject({
        method: 'POST',
        url: '/api/orgs',
        payload: { name: 'Acme2', slug: 'acme2' },
      });
      expect(response.statusCode).toBe(201);
    } finally {
      if (prev !== undefined) process.env['PROMPTSHEON_ALLOW_SYSTEM_ACTOR'] = prev;
      else delete process.env['PROMPTSHEON_ALLOW_SYSTEM_ACTOR'];
    }
  });
});