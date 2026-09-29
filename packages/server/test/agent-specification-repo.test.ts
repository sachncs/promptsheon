import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import Database from 'better-sqlite3';
import { applyMigrations, AgentSpecificationSchema, CasStore, type MigrationSql } from '@promptsheon/shared';
import { AgentSpecificationRepo } from '../src/repos/agent-specification.js';
import { mkdtemp, readFile, readdir, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const migrationsDir = join(dirname(fileURLToPath(import.meta.url)), '..', '..', 'shared', 'db', 'migrations');

async function loadMigrations(): Promise<MigrationSql[]> {
  const files = (await readdir(migrationsDir)).filter((file) => file.endsWith('.up.sql'));
  return Promise.all(files.map(async (file) => ({
    version: Number.parseInt(file.split('_')[0] ?? '0', 10),
    name: file,
    up: await readFile(join(migrationsDir, file), 'utf8'),
  })));
}

const specification = AgentSpecificationSchema.parse({
  role: 'Research assistant',
  objective: 'Answer questions with evidence.',
  prompt: { system: 'Be precise.' },
  modelPolicy: { provider: 'openai', model: 'gpt-5' },
  lifecycle: { owner: 'team-research' },
});

describe('AgentSpecificationRepo', () => {
  let db: Database.Database;
  let basePath: string;
  let repo: AgentSpecificationRepo;

  beforeEach(async () => {
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, await loadMigrations());
    db.prepare("INSERT INTO workspaces (id, name, organization, created_at, updated_at) VALUES ('ws1', 'One', '', ?, ?), ('ws2', 'Two', '', ?, ?)")
      .run('2026-01-01', '2026-01-01', '2026-01-01', '2026-01-01');
    basePath = await mkdtemp(join(tmpdir(), 'promptsheon-agent-spec-'));
    const cas = new CasStore(basePath);
    await cas.init();
    repo = new AgentSpecificationRepo(db, cas);
  });

  afterEach(async () => {
    db.close();
    await rm(basePath, { recursive: true, force: true });
  });

  it('deduplicates concurrent identical content while preserving tenant metadata', async () => {
    const records = await Promise.all(Array.from({ length: 8 }, () => repo.create({
      workspaceId: 'ws1', specification, author: 'alice', changeReason: 'initial',
    })));
    expect(new Set(records.map((record) => record.hash)).size).toBe(1);
    expect(db.prepare('SELECT COUNT(*) AS count FROM agent_specifications').get()).toEqual({ count: 1 });
    expect((await repo.get('ws1', records[0]!.hash)).specification).toEqual(specification);
    await expect(repo.get('ws2', records[0]!.hash)).rejects.toThrow('not found');
  });

  it('keeps revisions immutable and exposes parent lineage', async () => {
    const first = await repo.create({ workspaceId: 'ws1', specification, author: 'alice', changeReason: 'initial' });
    const second = await repo.create({
      workspaceId: 'ws1',
      specification: { ...specification, objective: 'Answer questions with sources.' },
      author: 'alice',
      changeReason: 'add sources',
      parentHash: first.hash,
    });
    expect(repo.listLineage('ws1', second.hash).map((record) => record.hash)).toEqual([first.hash, second.hash]);
    repo.publish('ws1', second.hash);
    expect((await repo.get('ws1', second.hash)).status).toBe('published');
    expect(() => repo.replace()).toThrow('immutable');
  });

  it('detects CAS corruption on read', async () => {
    const record = await repo.create({ workspaceId: 'ws1', specification, author: 'alice', changeReason: 'initial' });
    const objectPath = join(basePath, 'objects', record.hash.slice(0, 2), record.hash.slice(2));
    const bytes = await readFile(objectPath);
    const corrupted = Buffer.from(bytes);
    corrupted[corrupted.length - 1] = (corrupted[corrupted.length - 1] ?? 0) ^ 1;
    const { writeFile } = await import('node:fs/promises');
    await writeFile(objectPath, corrupted);
    await expect(repo.get('ws1', record.hash)).rejects.toThrow();
  });
});
