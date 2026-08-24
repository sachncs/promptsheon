import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import { registerManifestHashRoutes } from '../src/routes/manifest-hash.js';
import { ManifestRepo } from '../src/repos/manifest.js';
import { applyMigrations } from '@promptsheon/shared';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import type { Manifest } from '@promptsheon/shared';

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

function buildManifest(goal: string, capabilityId: string): Manifest {
  return {
    id: 'm1', version: 1,
    prompt: { systemPrompt: 'x', userTemplate: '{{input}}' },
    model: { provider: 'openai', modelId: 'gpt-4', temperature: 0.7, maxTokens: 100 },
    runtime: { timeoutMs: 1000, nodeTimeoutMs: 1000, totalTimeoutMs: 5000, maxRetries: 0, canaryPercent: 0, concurrencyLimit: 1 },
    context: { inputsSchema: {}, outputsSchema: {}, requiredContextVars: [] },
    memory: { enabled: false, type: 'stateless' },
    guardrails: { pre: [], post: [] },
    tools: [], mcpServers: [],
    evaluation: { datasets: [], scorers: [], passThreshold: 0.5 },
    nodes: [], edges: [],
    metadata: { goal, capabilityId, createdBy: 'tester' },
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  };
}

describe('POST /api/manifests (save/load)', () => {
  let app: FastifyInstance;
  let repo: ManifestRepo;
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
      VALUES ('proj1', 'ws1', 'p', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
    `).run();
    db.prepare(`
      INSERT INTO capabilities (id, project_id, name, description, created_at, updated_at)
      VALUES ('cap1', 'proj1', 'c', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
    `).run();
    repo = new ManifestRepo(db);
    app = Fastify();
    app.setErrorHandler((error, _request, reply) => {
      if (error.name === 'NotFoundError') return reply.code(404).send({ error: { code: 'NOT_FOUND', message: error.message } });
      if (error.statusCode) return reply.code(error.statusCode).send({ error: { code: 'APP_ERROR', message: error.message } });
      return reply.code(500).send({ error: { code: 'INTERNAL_ERROR', message: error.message } });
    });
    await app.register(async (instance) => {
      await registerManifestHashRoutes(instance, { manifestRepo: repo });
    });
    await app.ready();
  });

  afterEach(async () => {
    await app.close();
    db.close();
  });

  it('POST saves a manifest and returns the hash', async () => {
    const response = await app.inject({ method: 'POST', url: '/api/manifests', payload: buildManifest('test goal', 'cap1') });
    expect(response.statusCode).toBe(201);
    const body = response.json() as { hash: string };
    expect(body.hash).toMatch(/^[0-9a-f]{64}$/);
  });

  it('GET by hash returns the saved manifest', async () => {
    const created = (await app.inject({ method: 'POST', url: '/api/manifests', payload: buildManifest('test goal', 'cap1') })).json() as { hash: string };
    const response = await app.inject({ method: 'GET', url: `/api/manifests/${created.hash}` });
    expect(response.statusCode).toBe(200);
    const body = response.json() as { id: string; metadata: Record<string, unknown> };
    expect(body.id).toBe('m1');
    expect(body.metadata['goal']).toBe('test goal');
  });

  it('GET by unknown hash returns 404', async () => {
    const response = await app.inject({ method: 'GET', url: '/api/manifests/missing' });
    expect(response.statusCode).toBe(404);
  });

  it('POST with malformed body synthesizes a draft manifest and saves it as 201', async () => {
    const response = await app.inject({ method: 'POST', url: '/api/manifests', payload: { invalid: 'shape' } });
    expect(response.statusCode).toBe(201);
  });

  it('POST with non-object body is rejected (4xx)', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/api/manifests',
      payload: 'not-an-object',
      headers: { 'content-type': 'application/json' },
    });
    expect([400, 415, 422]).toContain(response.statusCode);
  });

  it('POST with nodes referencing invalid manifest shape returns 422', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/api/manifests',
      payload: { nodes: [{ id: 'n1', goal: 'x', manifest: 'not-an-object' }] },
    });
    expect(response.statusCode).toBe(422);
  });

  it('round-trip: save then load returns same manifest', async () => {
    const m = buildManifest('round-trip', 'cap1');
    const created = (await app.inject({ method: 'POST', url: '/api/manifests', payload: m })).json() as { hash: string };
    const response = await app.inject({ method: 'GET', url: `/api/manifests/${created.hash}` });
    const loaded = response.json() as { id: string; metadata: Record<string, unknown> };
    expect(loaded.id).toBe(m.id);
    expect(loaded.metadata['goal']).toBe('round-trip');
  });
});