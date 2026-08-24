import { describe, it, expect } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import Database from 'better-sqlite3';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { applyMigrations } from '@promptsheon/shared';
import { registerVersionRoutes } from '../src/routes/version.js';
import { VersionRepo } from '../src/repos/version.js';
import { ManifestRepo } from '../src/repos/manifest.js';

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

describe('GET /api/capability-versions/:versionId/manifest', () => {
  it('returns the manifest object stored on the version row', async () => {
    const db = openDb();
    db.prepare(
      `INSERT INTO orgs (id,name,slug,created_at,updated_at) VALUES ('o1','O','o',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
    ).run();
    db.prepare(
      `INSERT INTO workspaces (id,name,organization,created_at,updated_at) VALUES ('w1','W','O',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
    ).run();
    db.prepare(
      `INSERT INTO projects (id,workspace_id,name,description,created_at,updated_at) VALUES ('p1','w1','P','',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
    ).run();
    db.prepare(
      `INSERT INTO capabilities (id,project_id,name,description,created_at,updated_at) VALUES ('00000000-0000-4000-8000-000000000099','p1','C','',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
    ).run();

    const repo = new VersionRepo(db);
    const manifest = '{"nodes":[],"edges":[]}';
    const created = repo.create({
      capabilityId: '00000000-0000-4000-8000-000000000099',
      version: 1,
      manifest,
      manifestHash: 'unused',
      createdBy: 'u1',
    });
    const manifestRepo = new ManifestRepo(db);
    const app = Fastify({ logger: false });
    registerVersionRoutes(app, repo, manifestRepo);
    await app.ready();
    const r = await app.inject({ method: 'GET', url: `/api/capability-versions/${created.id}/manifest` });
    expect(r.statusCode).toBe(200);
    const body = r.json() as { manifest: unknown; size: number };
    expect(body.manifest).toEqual({ nodes: [], edges: [] });
    expect(body.size).toBe(manifest.length);
  });

  it('returns 404 for unknown id', async () => {
    const db = openDb();
    const manifestRepo = new ManifestRepo(db);
    const app = Fastify({ logger: false });
    registerVersionRoutes(app, new VersionRepo(db), manifestRepo);
    await app.ready();
    const r = await app.inject({ method: 'GET', url: '/api/capability-versions/missing/manifest' });
    expect(r.statusCode).toBe(404);
  });
});
