import { describe, it, expect, beforeEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import Database from 'better-sqlite3';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { applyMigrations } from '@promptsheon/shared';
import { registerApprovalRoutes } from '../src/routes/approval.js';
import { ApprovalRepo } from '../src/repos/approval.js';
import { ReleaseRepo } from '../src/repos/release.js';
import { ManifestRepo } from '../src/repos/manifest.js';
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

const ORG_ID = '00000000-0000-4000-8000-000000000001';
const PROJECT_ID = '00000000-0000-4000-8000-000000000002';
const CAP_ID = '00000000-0000-4000-8000-000000000003';
const VERSION_ID = '00000000-0000-4000-8000-000000000004';
const CREATOR = 'creator-user';
const VOTER = 'voter-user';

function seedRepoRelease(db: Database.Database, releaseRepo: ReleaseRepo, manifestRepo: ManifestRepo): string {
  const workspaceId = '00000000-0000-4000-8000-00000000a001';
  db.prepare(
    `INSERT INTO orgs (id, name, slug, created_at, updated_at) VALUES (?, 'O', 'o', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
  ).run(ORG_ID);
  db.prepare(
    `INSERT INTO workspaces (id, name, organization, created_at, updated_at)
     VALUES (?, 'W', 'O', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
  ).run(workspaceId);
  db.prepare(
    `INSERT INTO projects (id, workspace_id, name, description, created_at, updated_at)
     VALUES (?, ?, 'P', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
  ).run(PROJECT_ID, workspaceId);
  db.prepare(
    `INSERT INTO capabilities (id, project_id, name, description, created_at, updated_at)
     VALUES (?, ?, 'C', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
  ).run(CAP_ID, PROJECT_ID);
  db.prepare(
    `INSERT INTO capability_versions (id, capability_id, version, manifest, manifest_hash, created_at, created_by)
     VALUES (?, ?, 1, '{"nodes":[],"edges":[]}', '', CURRENT_TIMESTAMP, 'creator')`,
  ).run(VERSION_ID, CAP_ID);

  const releaseId = '00000000-0000-4000-8000-000000000b01';
  const manifest = '{"nodes":[],"edges":[]}';
  releaseRepo.create({
    capabilityId: CAP_ID,
    capabilityVersion: 1,
    capabilityVersionId: VERSION_ID,
    manifest,
    environment: 'dev',
    createdBy: CREATOR,
    canaryPercent: 0,
  });
  const stored = db.prepare('SELECT id FROM releases LIMIT 1').get() as { id: string };
  manifestRepo.registerFromRaw({
    capabilityId: CAP_ID,
    version: 1,
    manifestHash: releaseRepo.computeManifestHash(manifest),
    manifestJson: manifest,
    createdBy: CREATOR,
  });
  return stored.id;
}

describe('approval route reconciliation', () => {
  let app: FastifyInstance;
  let approvalRepo: ApprovalRepo;
  let releaseRepo: ReleaseRepo;
  let manifestRepo: ManifestRepo;
  let db: Database.Database;
  let releaseId: string;

  beforeEach(async () => {
    db = openDb();
    approvalRepo = new ApprovalRepo(db);
    releaseRepo = new ReleaseRepo(db);
    manifestRepo = new ManifestRepo(db);
    releaseId = seedRepoRelease(db, releaseRepo, manifestRepo);
    app = Fastify({ logger: false });
    app.setErrorHandler((err, _req, reply) => {
      if ((err as { name?: string }).name === 'NotFoundError') {
        return reply.code(404).send({ error: { code: 'NOT_FOUND', message: (err as Error).message } });
      }
      return reply.code(500).send({ error: { code: 'INTERNAL', message: (err as Error).message } });
    });
    app.addHook('preHandler', (request, _reply, done) => {
      (request as Record<string, unknown>)['userId'] = VOTER;
      done();
    });
    registerApprovalRoutes(app, approvalRepo, { releaseRepo, manifestRepo });
    await app.ready();
  });

  it('GET /api/approvals/:releaseId returns 404 when no row exists yet', async () => {
    const r = await app.inject({ method: 'GET', url: `/api/approvals/${releaseId}` });
    expect(r.statusCode).toBe(404);
  });

  it('GET /api/approvals?releaseId returns the row after a vote', async () => {
    await app.inject({
      method: 'POST',
      url: `/api/releases/${releaseId}/approvals`,
      payload: { decision: 'approve', comment: 'first' },
    });
    const r = await app.inject({ method: 'GET', url: `/api/approvals?releaseId=${releaseId}` });
    expect(r.statusCode).toBe(200);
    const body = r.json() as { releaseId: string };
    expect(body.releaseId).toBe(releaseId);
  });

  it('GET /api/approvals?releaseId=<id> returns empty when no row exists', async () => {
    const r = await app.inject({ method: 'GET', url: '/api/approvals?releaseId=nope' });
    expect(r.statusCode).toBe(200);
    const body = r.json() as { releaseId: string; approvals: unknown[] };
    expect(body.releaseId).toBe('nope');
    expect(body.approvals).toEqual([]);
  });

  it('GET /api/approvals without releaseId returns 400', async () => {
    const r = await app.inject({ method: 'GET', url: '/api/approvals' });
    expect(r.statusCode).toBe(400);
  });

  it('POST /api/releases/:releaseId/approvals records a vote', async () => {
    const r = await app.inject({
      method: 'POST',
      url: `/api/releases/${releaseId}/approvals`,
      payload: { decision: 'approve', comment: 'lgtm' },
    });
    expect(r.statusCode).toBe(201);
    const body = r.json() as { decision: string; distinctApprovers: number };
    expect(body.decision).toBe('approve');
    expect(body.distinctApprovers).toBe(1);
  });

  it('POST /api/releases/:unknown/approvals 404', async () => {
    const r = await app.inject({
      method: 'POST',
      url: '/api/releases/00000000-0000-4000-8000-deadbeef/approvals',
      payload: { decision: 'approve' },
    });
    expect(r.statusCode).toBe(404);
  });

  it('POST /api/releases/:releaseId/approvals rejects invalid decision', async () => {
    const r = await app.inject({
      method: 'POST',
      url: `/api/releases/${releaseId}/approvals`,
      payload: { decision: 'maybe' },
    });
    expect(r.statusCode).toBe(422);
  });

  it('legacy POST /api/approvals still upserts the votes blob', async () => {
    const r = await app.inject({
      method: 'POST',
      url: '/api/approvals',
      payload: { releaseId, votes: 'legacy-blob' },
    });
    expect(r.statusCode).toBe(201);
    const gotten = approvalRepo.getByReleaseId(releaseId);
    expect(gotten?.votes).toBe('legacy-blob');
  });
});
