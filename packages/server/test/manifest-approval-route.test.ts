import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import { registerManifestApprovalRoutes } from '../src/routes/manifest-approval.js';
import { AuditChain } from '../src/audit/chain.js';
import { ManifestRepo, computeManifestHash } from '../src/repos/manifest.js';
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

function buildManifest(): Manifest {
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
    nodes: [], edges: [], metadata: { capabilityId: 'cap1' },
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  };
}

describe('POST /api/manifests/:hash/approve|reject', () => {
  let app: FastifyInstance;
  let manifestRepo: ManifestRepo;
  let db: ReturnType<typeof import('better-sqlite3')>;
  let hash: string;

  beforeEach(async () => {
    const Database = (await import('better-sqlite3')).default;
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
    db.prepare(`
      INSERT INTO workspaces (id, name, organization, created_at, updated_at)
      VALUES ('ws1', 'Test WS', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
    `).run();
    db.prepare(`
      INSERT INTO projects (id, workspace_id, name, description, created_at, updated_at)
      VALUES ('proj1', 'ws1', 'Test Project', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
    `).run();
    db.prepare(`
      INSERT INTO capabilities (id, project_id, name, description, created_at, updated_at)
      VALUES ('cap1', 'proj1', 'Test', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
    `).run();

    manifestRepo = new ManifestRepo(db);
    const m = buildManifest();
    hash = manifestRepo.create(m, { goal: 'g', createdBy: 'alice' });
    void hash;

    app = Fastify();
    app.setErrorHandler((error, _request, reply) => {
      if (error.name === 'NotFoundError') {
        return reply.code(404).send({ error: { code: 'NOT_FOUND', message: error.message } });
      }
      if (error.statusCode) {
        return reply.code(error.statusCode).send({ error: { code: 'APP_ERROR', message: error.message } });
      }
      return reply.code(500).send({ error: { code: 'INTERNAL_ERROR', message: error.message } });
    });
    await app.register(async (instance) => {
      await registerManifestApprovalRoutes(instance, { manifestRepo, auditChain: new AuditChain(db) });
    });
    await app.ready();
  });

  afterEach(async () => {
    await app.close();
    db.close();
  });

  it('records an approval', async () => {
    const response = await app.inject({
      method: 'POST',
      url: `/api/manifests/${hash}/approve`,
      payload: { userId: 'user1', comment: 'lgtm' },
    });
    expect(response.statusCode).toBe(200);
    const body = response.json() as { distinctApprovers: number; approvals: Array<{ userId: string; vote: string; comment: string }> };
    expect(body.distinctApprovers).toBe(1);
    expect(body.approvals[0]).toEqual({ userId: 'user1', vote: 'approve', comment: 'lgtm', createdAt: expect.any(String) });
  });

  it('records a rejection', async () => {
    const response = await app.inject({
      method: 'POST',
      url: `/api/manifests/${hash}/reject`,
      payload: { userId: 'user1', comment: 'no' },
    });
    expect(response.statusCode).toBe(200);
    const body = response.json() as { distinctApprovers: number };
    expect(body.distinctApprovers).toBe(0);
  });

  it('counts 2 distinct approvers', async () => {
    await app.inject({ method: 'POST', url: `/api/manifests/${hash}/approve`, payload: { userId: 'user1' } });
    await app.inject({ method: 'POST', url: `/api/manifests/${hash}/approve`, payload: { userId: 'user2' } });
    const response = await app.inject({ method: 'GET', url: `/api/manifests/${hash}/approvals` });
    const body = response.json() as { distinctApprovers: number };
    expect(body.distinctApprovers).toBe(2);
  });

  it('overwrites prior vote on re-vote (distinctApprovers=0 if rejected)', async () => {
    await app.inject({ method: 'POST', url: `/api/manifests/${hash}/approve`, payload: { userId: 'user1' } });
    await app.inject({ method: 'POST', url: `/api/manifests/${hash}/reject`, payload: { userId: 'user1' } });
    const response = await app.inject({ method: 'GET', url: `/api/manifests/${hash}/approvals` });
    const body = response.json() as { distinctApprovers: number };
    expect(body.distinctApprovers).toBe(0);
  });

  it('returns 404 for unknown manifest', async () => {
    const response = await app.inject({ method: 'GET', url: '/api/manifests/nonexistent/approvals' });
    expect(response.statusCode).toBe(404);
  });

  it('returns 422 when userId is missing', async () => {
    const response = await app.inject({ method: 'POST', url: `/api/manifests/${hash}/approve`, payload: {} });
    expect(response.statusCode).toBe(422);
  });
});