import { describe, it, expect } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import Database from 'better-sqlite3';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { applyMigrations } from '@promptsheon/shared';
import { registerExecutionRoutes } from '../src/routes/execution.js';
import { ExecutionRepo } from '../src/repos/execution.js';
import { ReleaseRepo } from '../src/repos/release.js';
import { ManifestRepo } from '../src/repos/manifest.js';
import { VersionRepo } from '../src/repos/version.js';
import type { ManifestGraphExecutor } from '../src/agents/executor/index.js';

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

const ORG = '00000000-0000-4000-8000-000000000001';
const WS = '00000000-0000-4000-8000-00000000000a';
const PROJECT = '00000000-0000-4000-8000-00000000000b';
const CAP = '00000000-0000-4000-8000-00000000000c';

function seed(db: Database.Database) {
  db.prepare(
    `INSERT INTO orgs (id,name,slug,created_at,updated_at) VALUES (?, 'O','o',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
  ).run(ORG);
  db.prepare(
    `INSERT INTO workspaces (id,name,organization,created_at,updated_at) VALUES (?, 'W','O',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
  ).run(WS);
  db.prepare(
    `INSERT INTO projects (id,workspace_id,name,description,created_at,updated_at) VALUES (?, ?, 'P','',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
  ).run(PROJECT, WS);
  db.prepare(
    `INSERT INTO capabilities (id,project_id,name,description,created_at,updated_at) VALUES (?, ?, 'C','',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
  ).run(CAP, PROJECT);
}

class StubExecutor {
  async execute(_hash: string, _manifest: unknown) {
    return {
      nodeResults: [],
      totalLatencyMs: 0,
      totalCost: 0,
      totalTokens: 0,
      error: null,
    };
  }
}

describe('/api/invoke alias', () => {
  it('returns 404 when capabilityVersionId is unknown', async () => {
    const db = openDb();
    seed(db);
    const app = Fastify({ logger: false });
    registerExecutionRoutes(app, {
      executionRepo: new ExecutionRepo(db),
      releaseRepo: new ReleaseRepo(db),
      manifestRepo: new ManifestRepo(db),
      versionRepo: new VersionRepo(db),
      executor: new StubExecutor() as unknown as ManifestGraphExecutor,
    });
    await app.ready();
    const r = await app.inject({
      method: 'POST',
      url: '/api/invoke',
      payload: { capabilityVersionId: 'unknown', inputs: {} },
    });
    expect(r.statusCode).toBe(404);
    expect(r.json().error.code).toBe('VERSION_NOT_FOUND');
  });

  it('returns 404 when version has no manifestHash', async () => {
    const db = openDb();
    seed(db);
    db.prepare(
      `INSERT INTO capability_versions (id, capability_id, version, manifest, manifest_hash, created_at, created_by)
       VALUES ('v1', ?, 1, '{}', '', CURRENT_TIMESTAMP, 'u1')`,
    ).run(CAP);
    const app = Fastify({ logger: false });
    registerExecutionRoutes(app, {
      executionRepo: new ExecutionRepo(db),
      releaseRepo: new ReleaseRepo(db),
      manifestRepo: new ManifestRepo(db),
      versionRepo: new VersionRepo(db),
      executor: new StubExecutor() as unknown as ManifestGraphExecutor,
    });
    await app.ready();
    const r = await app.inject({
      method: 'POST',
      url: '/api/invoke',
      payload: { capabilityVersionId: 'v1', inputs: {} },
    });
    expect(r.statusCode).toBe(409);
    expect(r.json().error.code).toBe('NO_MANIFEST_HASH');
  });
});
