import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import { registerReleaseRoutes } from '../src/routes/release.js';
import { ManifestRepo } from '../src/repos/manifest.js';
import { ReleaseRepo } from '../src/repos/release.js';
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

function makeRelease(repo: ReleaseRepo, capabilityId: string, environment: string, version: number, createdBy: string): string {
  const result = repo.create({
    capabilityId,
    capabilityVersion: version,
    capabilityVersionId: null,
    manifest: JSON.stringify({ systemPrompt: 'x' }),
    environment,
    createdBy,
    canaryPercent: 0,
  });
  return (result as { id: string }).id;
}

describe('POST /api/releases/:id/rollback', () => {
  let app: FastifyInstance;
  let repo: ReleaseRepo;
  let db: ReturnType<typeof import('better-sqlite3')>;

  beforeEach(async () => {
    const Database = (await import('better-sqlite3')).default;
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
    db.prepare(`
      INSERT INTO workspaces (id, name, organization, created_at, updated_at)
      VALUES ('ws1', 'ws', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
    `).run();
    db.prepare(`
      INSERT INTO projects (id, workspace_id, name, description, created_at, updated_at)
      VALUES ('proj1', 'ws1', 'P', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
    `).run();
    db.prepare(`
      INSERT INTO capabilities (id, project_id, name, description, created_at, updated_at)
      VALUES ('cap1', 'proj1', 'C', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
    `).run();
    db.prepare(`
      INSERT INTO capability_versions (id, capability_id, version, manifest, created_at)
      VALUES ('cv1', 'cap1', 1, '{}', '2026-01-01T00:00:00Z'),
             ('cv2', 'cap1', 2, '{}', '2026-01-01T00:00:00Z'),
             ('cv3', 'cap1', 3, '{}', '2026-01-01T00:00:00Z')
    `).run();
    repo = new ReleaseRepo(db);
    app = Fastify();
    app.setErrorHandler((error, _request, reply) => {
      if (error.statusCode) {
        return reply.code(error.statusCode).send({ error: { code: 'APP_ERROR', message: error.message } });
      }
      return reply.code(500).send({ error: { code: 'INTERNAL_ERROR', message: error.message } });
    });
    await app.register(async (instance) => {
      await registerReleaseRoutes(instance, repo, { manifestRepo: new ManifestRepo(db), auditChain: new AuditChain(db) });
    });
    await app.ready();
  });

  afterEach(async () => {
    await app.close();
    db.close();
  });

  it('rollback to most recent superseded release', async () => {
    const v1 = makeRelease(repo, 'cap1', 'prod', 1, 'alice');
    const v2 = makeRelease(repo, 'cap1', 'prod', 2, 'alice');
    const v3 = makeRelease(repo, 'cap1', 'prod', 3, 'alice');
    repo.updateStatus(v1, 'active');
    repo.updateStatus(v1, 'superseded');
    repo.updateStatus(v2, 'active');
    repo.updateStatus(v2, 'superseded');
    repo.updateStatus(v3, 'active');
    void v1;

    const response = await app.inject({ method: 'POST', url: `/api/releases/${v3}/rollback`, payload: {} });
    expect(response.statusCode).toBe(200);
    const body = response.json() as { reactivated: { id: string; status: string }; superseded: { id: string; status: string } };
    expect(body.reactivated.id).toBe(v2);
    expect(body.reactivated.status).toBe('active');
    expect(body.superseded.id).toBe(v3);
    expect(body.superseded.status).toBe('superseded');
  });

  it('rollback to specific release by toReleaseId', async () => {
    const v1 = makeRelease(repo, 'cap1', 'prod', 1, 'alice');
    const v2 = makeRelease(repo, 'cap1', 'prod', 2, 'alice');
    const v3 = makeRelease(repo, 'cap1', 'prod', 3, 'alice');
    repo.updateStatus(v1, 'active');
    repo.updateStatus(v1, 'superseded');
    repo.updateStatus(v2, 'active');
    repo.updateStatus(v2, 'superseded');
    repo.updateStatus(v3, 'active');
    void v2;

    const response = await app.inject({
      method: 'POST',
      url: `/api/releases/${v3}/rollback`,
      payload: { toReleaseId: v1 },
    });
    expect(response.statusCode).toBe(200);
    const body = response.json() as { reactivated: { id: string } };
    expect(body.reactivated.id).toBe(v1);
  });

  it('returns 404 when no previous release exists', async () => {
    const v1 = makeRelease(repo, 'cap1', 'prod', 1, 'alice');
    repo.updateStatus(v1, 'active');
    const response = await app.inject({ method: 'POST', url: `/api/releases/${v1}/rollback`, payload: {} });
    expect(response.statusCode).toBe(404);
  });

  it('returns 404 when target release does not exist', async () => {
    const v1 = makeRelease(repo, 'cap1', 'prod', 1, 'alice');
    repo.updateStatus(v1, 'active');
    const response = await app.inject({ method: 'POST', url: `/api/releases/${v1}/rollback`, payload: {} });
    expect(response.statusCode).toBe(404);
  });

  it('canary percent update returns updated release', async () => {
    const v1 = makeRelease(repo, 'cap1', 'prod', 1, 'alice');
    const response = await app.inject({
      method: 'PUT',
      url: `/api/releases/${v1}/canary`,
      payload: { percent: 30 },
    });
    expect(response.statusCode).toBe(200);
    const body = response.json() as { canaryPercent: number };
    expect(body.canaryPercent).toBe(30);
  });
});