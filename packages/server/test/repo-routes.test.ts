import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import Database from 'better-sqlite3';
import { applyMigrations } from '@promptsheon/shared';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { RepoRepo } from '../src/repos/repo.js';
import { BranchRepo } from '../src/repos/branch.js';
import { TagRepo } from '../src/repos/tag.js';
import { RepoStore } from '../src/repos/repo-store.js';
import { CommitRepo } from '../src/repos/commit.js';
import { registerRepoRoutes } from '../src/routes/repo.js';
import { registerContentsRoutes } from '../src/routes/contents.js';
import { registerCommitRoutes } from '../src/routes/commits.js';

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

const WS_ID = 'a3c642fe-b277-412c-a6e6-3a8c66f68589';

describe('repository / branch / file / commit round-trip', () => {
  let app: FastifyInstance;
  let db: Database.Database;
  let repoRepo: RepoRepo;
  let branchRepo: BranchRepo;
  let tagRepo: TagRepo;
  let repoStore: RepoStore;
  let commitRepo: CommitRepo;

  beforeEach(async () => {
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
    db.prepare(
      `INSERT INTO workspaces (id, name, organization, created_at, updated_at) VALUES (?, 'ws', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
    ).run(WS_ID);
    db.prepare(
      `INSERT INTO users (id, email, name, role, created_at, updated_at) VALUES ('system', 's@s.l', 'system', 'admin', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
    ).run();
    repoRepo = new RepoRepo(db);
    branchRepo = new BranchRepo(db);
    tagRepo = new TagRepo(db);
    repoStore = new RepoStore(db);
    commitRepo = new CommitRepo(db);

    app = Fastify();
    app.setErrorHandler((error, _request, reply) => {
      if (error.name === 'NotFoundError') return reply.code(404).send({ error: { code: 'NOT_FOUND', message: error.message } });
      if (error.statusCode) return reply.code(error.statusCode).send({ error: { code: 'APP_ERROR', message: error.message } });
      return reply.code(500).send({ error: { code: 'INTERNAL_ERROR', message: String(error) } });
    });
    await app.register(async (instance) => {
      await registerRepoRoutes(instance, { repoRepo, branchRepo, tagRepo });
      await registerContentsRoutes(instance, { repoRepo, branchRepo, repoStore });
      await registerCommitRoutes(instance, { repoRepo, branchRepo, repoStore, commitRepo });
    });
    await app.ready();
  });

  afterEach(async () => {
    await app.close();
    db.close();
  });

  it('creates a repo and auto-creates the default branch', async () => {
    const res = await app.inject({
      method: 'POST',
      url: '/api/repos',
      payload: { workspaceId: WS_ID, name: 'refund-triage', minApprovers: 1, requireSignedReleases: false },
    });
    expect(res.statusCode).toBe(201);
    const repo = res.json() as { id: string; defaultBranch: string };
    expect(repo.defaultBranch).toBe('main');

    const branches = await app.inject({ method: 'GET', url: `/api/repos/${repo.id}/branches` });
    expect(branches.statusCode).toBe(200);
    const list = branches.json() as Array<{ name: string }>;
    expect(list.find((b) => b.name === 'main')).toBeDefined();
  });

  it('rejects duplicate repo creation with 409', async () => {
    const create = { workspaceId: WS_ID, name: 'repo-x' };
    const first = await app.inject({ method: 'POST', url: '/api/repos', payload: create });
    expect(first.statusCode).toBe(201);
    const dup = await app.inject({ method: 'POST', url: '/api/repos', payload: create });
    expect(dup.statusCode).toBe(409);
  });

  it('refuses to delete the default branch', async () => {
    const created = (
      await app.inject({
        method: 'POST',
        url: '/api/repos',
        payload: { workspaceId: WS_ID, name: 'r' },
      })
    ).json() as { id: string };
    const res = await app.inject({ method: 'DELETE', url: `/api/repos/${created.id}/branches/main` });
    expect(res.statusCode).toBe(422);
  });

  it('round-trip: stage a file, commit, read content back', async () => {
    const created = (
      await app.inject({
        method: 'POST',
        url: '/api/repos',
        payload: { workspaceId: WS_ID, name: 'r' },
      })
    ).json() as { id: string };

    const put = await app.inject({
      method: 'PUT',
      url: `/api/repos/${created.id}/contents/prompts/main.md?ref=main`,
      payload: { path: 'prompts/main.md', content: 'You are an assistant.', ref: 'main' },
    });
    expect(put.statusCode).toBe(200);
    const blob = put.json() as { blobOid: string };
    expect(blob.blobOid).toMatch(/^[0-9a-f]{64}$/);

    const commit = await app.inject({
      method: 'POST',
      url: `/api/repos/${created.id}/commits`,
      payload: { ref: 'main', message: 'first commit' },
    });
    expect(commit.statusCode).toBe(201);
    const c = commit.json() as { oid: string; treeOid: string; parents: string[] };
    expect(c.parents).toEqual([]);
    expect(c.treeOid).toMatch(/^[0-9a-f]{64}$/);

    const get = await app.inject({
      method: 'GET',
      url: `/api/repos/${created.id}/contents/prompts/main.md?ref=main`,
    });
    expect(get.statusCode).toBe(200);
    const read = get.json() as { content: string; blobOid: string };
    expect(read.content).toBe('You are an assistant.');
    expect(read.blobOid).toBe(blob.blobOid);

    const list = await app.inject({
      method: 'GET',
      url: `/api/repos/${created.id}/commits?ref=main`,
    });
    const commits = list.json() as Array<{ oid: string; message: string }>;
    expect(commits.length).toBe(1);
    expect(commits[0]?.message).toBe('first commit');
  });

  it('same content twice → same blob oid (deduplication)', async () => {
    const created = (
      await app.inject({
        method: 'POST',
        url: '/api/repos',
        payload: { workspaceId: WS_ID, name: 'r' },
      })
    ).json() as { id: string };
    const a = (
      await app.inject({
        method: 'PUT',
        url: `/api/repos/${created.id}/contents/file.md?ref=main`,
        payload: { path: 'file.md', content: 'hello', ref: 'main' },
      })
    ).json() as { blobOid: string };
    const b = (
      await app.inject({
        method: 'PUT',
        url: `/api/repos/${created.id}/contents/other.md?ref=main`,
        payload: { path: 'other.md', content: 'hello', ref: 'main' },
      })
    ).json() as { blobOid: string };
    expect(a.blobOid).toBe(b.blobOid);
  });

  it('normalises an upstream "../" path so it cannot escape the repo', async () => {
    const created = (
      await app.inject({
        method: 'POST',
        url: '/api/repos',
        payload: { workspaceId: WS_ID, name: 'r' },
      })
    ).json() as { id: string };
    const res = await app.inject({
      method: 'PUT',
      url: `/api/repos/${created.id}/contents/x?ref=main`,
      payload: { path: '../etc/passwd', content: 'x', ref: 'main' },
    });
    // The URL path is `/x` so it stores under the repo root regardless
    // of what the body says — `pathRaw` from the URL wins by design.
    expect(res.statusCode).toBe(200);
    const list = (
      await app.inject({
        method: 'GET',
        url: `/api/repos/${created.id}/contents?ref=main`,
      })
    ).json() as Array<{ path: string }>;
    expect(list.map((e) => e.path)).toEqual(['x']);
  });
});
