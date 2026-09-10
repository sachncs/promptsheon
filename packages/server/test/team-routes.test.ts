import { describe, it, expect, beforeEach } from 'vitest';
import Database from 'better-sqlite3';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { applyMigrations } from '@promptsheon/shared';
import Fastify, { type FastifyInstance } from 'fastify';
import { registerTeamRoutes } from '../src/routes/team.js';
import { TeamRepo, SsoConfigRepo } from '../src/repos/team.js';
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

function buildApp(role: 'admin' | 'reader'): { app: FastifyInstance; db: Database.Database } {
  const db = openDb();
  const teamRepo = new TeamRepo(db);
  const ssoRepo = new SsoConfigRepo(db);
  const audit = new AuditChain(db);
  db.prepare(
    `INSERT OR IGNORE INTO orgs (id, name, slug, created_at, updated_at)
     VALUES ('00000000-0000-4000-8000-000000000001', 'Test Org', 'test-org', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
  ).run();
  const app = Fastify({ logger: false });
  app.addHook('preHandler', (request, _reply, done) => {
    (request as Record<string, unknown>)['userId'] = 'u-test';
    (request as Record<string, unknown>)['orgContext'] = {
      organizationId: '00000000-0000-4000-8000-000000000001',
      role,
    };
    done();
  });
  registerTeamRoutes(app, {
    teamRepo,
    ssoConfigRepo: ssoRepo,
    auditChain: audit,
    scimBearerToken: 'test-scim-token',
  });
  return { app, db };
}

describe('Team + SCIM routes', () => {
  describe('teams CRUD', () => {
    it('admin can create + list teams', async () => {
      const { app, db } = buildApp('admin');
      const r1 = await app.inject({
        method: 'POST',
        url: '/api/teams',
        payload: { name: 'Core', slug: 'core', description: 'core team' },
      });
      if (r1.statusCode !== 201) {
        process.stderr.write('r1 body: ' + r1.body + '\n');
      }
      expect(r1.statusCode).toBe(201);
      const team = r1.json() as { id: string; slug: string };
      expect(team.slug).toBe('core');

      const r2 = await app.inject({ method: 'GET', url: '/api/teams' });
      const body = r2.json() as { items: Array<unknown> };
      expect(body.items.length).toBe(1);
      expect(db.prepare('SELECT COUNT(*) AS c FROM teams').get()).toEqual({ c: 1 });
    });

    it('reader cannot create a team', async () => {
      const { app } = buildApp('reader');
      const r = await app.inject({
        method: 'POST',
        url: '/api/teams',
        payload: { name: 'X', slug: 'x' },
      });
      expect(r.statusCode).toBe(403);
    });

    it('rejects bad slug format', async () => {
      const { app } = buildApp('admin');
      const r = await app.inject({
        method: 'POST',
        url: '/api/teams',
        payload: { name: 'X', slug: 'Has Space' },
      });
      expect(r.statusCode).toBe(422);
    });
  });

  describe('team membership', () => {
    let teamId: string;
    beforeEach(async () => {
      const { app, db } = buildApp('admin');
      await db.prepare(
        `INSERT INTO users (id, email, name, role, created_at, updated_at) VALUES ('u1', 'a@b.test', 'A', 'member', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
      ).run();
      const t = db
        .prepare(`INSERT OR IGNORE INTO orgs (id,name,slug,created_at,updated_at) VALUES ('org-1','O','o',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`)
        .run();
      void t;
      teamId = db
        .prepare(
          `INSERT INTO teams (id, org_id, organisation_id, name, slug, description, created_at, updated_at) VALUES ('t1','org-1','org-1','Core','core','',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
        )
        .run().lastInsertRowid as string;
      const r = await app.inject({
        method: 'POST',
        url: '/api/teams/t1/members',
        payload: { userId: 'u1', role: 'member' },
      });
      void r;
    });

    it('admin can add and remove members', async () => {
      const { app, db } = buildApp('admin');
      await db.prepare(
        `INSERT OR IGNORE INTO orgs (id, name, slug, created_at, updated_at) VALUES ('org-1','Org 1','org-1',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
      ).run();
      await db.prepare(
        `INSERT INTO users (id, email, name, role, created_at, updated_at) VALUES ('u1', 'a@b.test', 'A', 'member', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
      ).run();
      await db.prepare(
        `INSERT INTO teams (id, org_id, organisation_id, name, slug, description, created_at, updated_at) VALUES ('t1','org-1','org-1','Core','core','',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
      ).run();
      const add = await app.inject({
        method: 'POST',
        url: '/api/teams/t1/members',
        payload: { userId: 'u1', role: 'admin' },
      });
      if (add.statusCode !== 201) {
        process.stderr.write('add body: ' + add.body + '\n');
      }
      expect(add.statusCode).toBe(201);
      const list = db.prepare('SELECT role FROM team_members WHERE team_id = ? AND user_id = ?').get('t1', 'u1') as { role: string } | undefined;
      expect(list?.role).toBe('admin');

      const remove = await app.inject({
        method: 'DELETE',
        url: '/api/teams/t1/members/u1',
      });
      expect(remove.statusCode).toBe(204);
      expect(db.prepare('SELECT COUNT(*) AS c FROM team_members').get()).toEqual({ c: 0 });
      void teamId;
    });
  });

  describe('effective role', () => {
    it('org role wins when no team', () => {
      const db = openDb();
      const repo = new TeamRepo(db);
      expect(repo.effectiveRole('u1', 'org-1', 'admin')).toBe('admin');
      expect(repo.effectiveRole('u1', 'org-1', 'reader')).toBe('reader');
    });

    it('higher team role overrides org role', () => {
      const db = openDb();
      db.prepare(
        `INSERT OR IGNORE INTO orgs (id,name,slug,created_at,updated_at) VALUES ('org-1','O','o',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
      ).run();
      db.prepare(
        `INSERT OR IGNORE INTO users (id, email, name, role, org_id, created_at, updated_at) VALUES ('u1', 'a@b.test', 'A', 'reader', 'org-1', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
      ).run();
      db.prepare(
        `INSERT OR IGNORE INTO orgs (id,name,slug,created_at,updated_at) VALUES ('org-1','O','o',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
      ).run();
      db.prepare(
        `INSERT OR IGNORE INTO users (id, email, name, role, org_id, created_at, updated_at) VALUES ('u-teamrole', 'a-teamrole@b.test', 'A', 'reader', 'org-1', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
      ).run();
      db.prepare(
        `INSERT OR IGNORE INTO teams (id, org_id, organisation_id, name, slug, description, created_at, updated_at) VALUES ('t1','org-1','org-1','Core','core','',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
      ).run();
      const repo = new TeamRepo(db);
      repo.addMember('t1', 'u-teamrole', 'admin');
      expect(repo.effectiveRole('u-teamrole', 'org-1', 'reader')).toBe('admin');
    });
  });

  describe('SCIM 2.0 provisioning', () => {
    it('rejects unauthenticated requests', async () => {
      const { app } = buildApp('admin');
      const r = await app.inject({ method: 'GET', url: '/api/scim/v2/Users' });
      expect(r.statusCode).toBe(401);
    });

    it('accepts requests with the SCIM bearer token', async () => {
      const { app } = buildApp('admin');
      const r = await app.inject({
        method: 'POST',
        url: '/api/scim/v2/Users',
        headers: { authorization: 'Bearer test-scim-token' },
        payload: {
          userName: 'alice',
          emails: [{ value: 'alice@corp.test', primary: true }],
          active: true,
        },
      });
      expect(r.statusCode).toBe(201);
      const body = r.json() as { id: string; schemas: string[] };
      expect(body.id).toMatch(/^scim-/);
      expect(body.schemas).toContain('urn:ietf:params:scim:schemas:core:2.0:User');
    });

    it('rejects requests with the wrong SCIM bearer token', async () => {
      const { app } = buildApp('admin');
      const r = await app.inject({
        method: 'GET',
        url: '/api/scim/v2/Users',
        headers: { authorization: 'Bearer wrong-token' },
      });
      expect(r.statusCode).toBe(401);
    });

    it('rejects POST without at least one email', async () => {
      const { app } = buildApp('admin');
      const r = await app.inject({
        method: 'POST',
        url: '/api/scim/v2/Users',
        headers: { authorization: 'Bearer test-scim-token' },
        payload: { userName: 'bob' },
      });
      // Zod rejects the missing emails array with 422.
      expect([400, 422]).toContain(r.statusCode);
    });
  });

  describe('SSO config', () => {
    it('admin can set the OIDC config; secret is not echoed back', async () => {
      const { app, db } = buildApp('admin');
      const r = await app.inject({
        method: 'POST',
        url: '/api/auth/oidc/config',
        payload: {
          provider: 'okta',
          issuer: 'https://corp.okta.com',
          clientId: 'client-abc',
          clientSecret: 'super-secret',
        },
      });
      expect(r.statusCode).toBe(200);
      const stored = db
        .prepare(
          `SELECT client_secret_encrypted FROM sso_configs WHERE organization_id = ?`,
        )
        .get('00000000-0000-4000-8000-000000000001') as { client_secret_encrypted: string };
      expect(stored.client_secret_encrypted).not.toBe('super-secret');
      const get = await app.inject({ method: 'GET', url: '/api/auth/oidc/config' });
      expect(get.statusCode).toBe(200);
      const config = get.json() as { configured: boolean; clientSecretEncrypted?: string; clientSecret?: string };
      expect(config.configured).toBe(true);
      expect(config.clientSecret).toBeUndefined();
    });
  });
});
