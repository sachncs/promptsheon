import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import { registerOrgTeamRoutes } from '../src/routes/org-team.js';
import { OrgRepo, TeamRepo, MembershipRepo } from '../src/repos/org.js';
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

describe('Org/Team CRUD routes', () => {
  let app: FastifyInstance;
  let db: ReturnType<typeof import('better-sqlite3')>;
  let orgRepo: OrgRepo;
  let teamRepo: TeamRepo;
  let membershipRepo: MembershipRepo;

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

  describe('POST /api/orgs', () => {
    it('creates an org', async () => {
      const response = await app.inject({ method: 'POST', url: '/api/orgs', payload: { name: 'Acme', slug: 'acme' } });
      expect(response.statusCode).toBe(201);
      const body = response.json() as { slug: string };
      expect(body.slug).toBe('acme');
    });

    it('rejects duplicate slug with 409', async () => {
      await app.inject({ method: 'POST', url: '/api/orgs', payload: { name: 'A', slug: 'same' } });
      const response = await app.inject({ method: 'POST', url: '/api/orgs', payload: { name: 'B', slug: 'same' } });
      expect(response.statusCode).toBe(409);
    });

    it('returns 422 for invalid slug pattern', async () => {
      const response = await app.inject({ method: 'POST', url: '/api/orgs', payload: { name: 'A', slug: 'INVALID!' } });
      expect(response.statusCode).toBe(422);
    });
  });

  describe('GET /api/orgs/:id', () => {
    it('returns org by id', async () => {
      const created = (await app.inject({ method: 'POST', url: '/api/orgs', payload: { name: 'A', slug: 'a' } })).json() as { id: string };
      const response = await app.inject({ method: 'GET', url: `/api/orgs/${created.id}` });
      expect(response.statusCode).toBe(200);
    });

    it('returns 404 for missing', async () => {
      const response = await app.inject({ method: 'GET', url: '/api/orgs/nonexistent' });
      expect(response.statusCode).toBe(404);
    });
  });

  describe('PUT /api/orgs/:id', () => {
    it('updates org name', async () => {
      const created = (await app.inject({ method: 'POST', url: '/api/orgs', payload: { name: 'A', slug: 'a' } })).json() as { id: string };
      const response = await app.inject({ method: 'PUT', url: `/api/orgs/${created.id}`, payload: { name: 'B' } });
      expect(response.statusCode).toBe(200);
      const body = response.json() as { name: string };
      expect(body.name).toBe('B');
    });
  });

  describe('Members', () => {
    it('adds, lists, removes an org member', async () => {
      const created = (await app.inject({ method: 'POST', url: '/api/orgs', payload: { name: 'A', slug: 'a' } })).json() as { id: string };
      db.prepare(`INSERT INTO users (id, org_id, email, name, role, created_at, updated_at) VALUES ('u1', '${created.id}', 'a@b.com', 'A', 'viewer', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`).run();
      const addRes = await app.inject({ method: 'POST', url: `/api/orgs/${created.id}/members`, payload: { userId: 'u1', role: 'admin' } });
      expect(addRes.statusCode).toBe(201);
      const listRes = await app.inject({ method: 'GET', url: `/api/orgs/${created.id}/members` });
      const body = listRes.json() as { members: Array<unknown> };
      expect(body.members.length).toBe(1);
      const delRes = await app.inject({ method: 'DELETE', url: `/api/orgs/${created.id}/members/u1` });
      expect(delRes.statusCode).toBe(204);
    });

    it('returns 404 when removing non-existent member', async () => {
      const created = (await app.inject({ method: 'POST', url: '/api/orgs', payload: { name: 'A', slug: 'a' } })).json() as { id: string };
      const response = await app.inject({ method: 'DELETE', url: `/api/orgs/${created.id}/members/nonexistent` });
      expect(response.statusCode).toBe(404);
    });
  });

  describe('Teams', () => {
    it('adds and lists teams', async () => {
      const org = (await app.inject({ method: 'POST', url: '/api/orgs', payload: { name: 'A', slug: 'a' } })).json() as { id: string };
      const createRes = await app.inject({ method: 'POST', url: `/api/orgs/${org.id}/teams`, payload: { name: 'Eng' } });
      expect(createRes.statusCode).toBe(201);
      const listRes = await app.inject({ method: 'GET', url: `/api/orgs/${org.id}/teams` });
      const body = listRes.json() as { teams: Array<{ name: string }> };
      expect(body.teams).toHaveLength(1);
      expect(body.teams[0].name).toBe('Eng');
    });
  });
});