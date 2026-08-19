import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import { registerReleaseRoutes, approvalGate } from '../src/routes/release.js';
import { ReleaseRepo } from '../src/repos/release.js';
import { ManifestRepo } from '../src/repos/manifest.js';
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
      return { version, name: f ?? 'unknown', up };
    })
    .filter((m) => m.version !== 0)
    .sort((a, b) => a.version - b.version);
}

describe('approvalGate (pure logic)', () => {
  function makeMockRepo(approverIds: string[]): any {
    return {
      computeManifestHash: (_: string) => 'h',
      findApprovals: () => approverIds.map((userId) => ({ userId, vote: 'approve', comment: '', createdAt: '' })),
    };
  }

  it('allows when 2+ distinct approvers (different from creator)', () => {
    const result = approvalGate({ createdBy: 'alice', manifest: '{}' }, makeMockRepo(['bob', 'carol']));
    expect(result).toBeNull();
  });

  it('blocks when creator is approver (maker-checker)', () => {
    const result = approvalGate({ createdBy: 'alice', manifest: '{}' }, makeMockRepo(['alice', 'bob']));
    expect(result).toContain('maker-checker');
  });

  it('blocks when fewer than 2 approvers', () => {
    const result = approvalGate({ createdBy: 'alice', manifest: '{}' }, makeMockRepo(['bob']));
    expect(result).toContain('insufficient');
  });

  it('blocks when no approvers', () => {
    const result = approvalGate({ createdBy: 'alice', manifest: '{}' }, makeMockRepo([]));
    expect(result).toContain('insufficient');
  });
});

describe('PUT /api/releases/:id/activate (approval gate)', () => {
  let app: FastifyInstance;
  let db: ReturnType<typeof import('better-sqlite3')>;
  let releaseRepo: ReleaseRepo;
  let manifestRepo: ManifestRepo;

  beforeEach(async () => {
    const Database = (await import('better-sqlite3')).default;
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
    db.prepare(`INSERT INTO workspaces (id, name, organization, created_at, updated_at) VALUES ('ws1', 'ws', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`).run();
    db.prepare(`INSERT INTO projects (id, workspace_id, name, description, created_at, updated_at) VALUES ('proj1', 'ws1', 'p', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`).run();
    db.prepare(`INSERT INTO capabilities (id, project_id, name, description, created_at, updated_at) VALUES ('cap1', 'proj1', 'c', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`).run();
    releaseRepo = new ReleaseRepo(db);
    manifestRepo = new ManifestRepo(db);

    app = Fastify({ logger: false });
    app.setErrorHandler((error, _request, reply) => {
      if (error.statusCode) return reply.code(error.statusCode).send({ error: { code: 'APP_ERROR', message: error.message } });
      return reply.code(500).send({ error: { code: 'INTERNAL_ERROR', message: error.message } });
    });
    await app.register(async (instance) => {
      await registerReleaseRoutes(instance, releaseRepo, { manifestRepo, auditChain: new AuditChain(db) });
    });
    await app.ready();
  });

  afterEach(async () => {
    await app.close();
    db.close();
  });

  function seedApprovedRelease(creator: string, approvers: string[]): string {
    const manifest = {
      id: 'm1', version: 1,
      prompt: { systemPrompt: 'x', userTemplate: '{{input}}' },
      model: { provider: 'openai', modelId: 'gpt-4', temperature: 0.7, maxTokens: 100 },
      runtime: { timeoutMs: 1000, nodeTimeoutMs: 1000, totalTimeoutMs: 1000, maxRetries: 0, canaryPercent: 0, concurrencyLimit: 1 },
      context: { inputsSchema: {}, outputsSchema: {}, requiredContextVars: [] },
      memory: { enabled: false, type: 'stateless' as const },
      guardrails: { pre: [], post: [] },
      tools: [], mcpServers: [],
      evaluation: { datasets: [], scorers: [], passThreshold: 0.5 },
      nodes: [], edges: [],
      metadata: { capabilityId: 'cap1' },
      createdAt: '2026-01-01T00:00:00Z',
      updatedAt: '2026-01-01T00:00:00Z',
    };
    const manifestHash = manifestRepo.create(manifest, { goal: 'g', createdBy: creator });
    for (const approver of approvers) {
      manifestRepo.upsertApproval(manifestHash, approver, 'approve');
    }
    const release = releaseRepo.create({
      capabilityId: 'cap1', capabilityVersion: 1, capabilityVersionId: null,
      manifest: JSON.stringify(manifest), environment: 'prod', createdBy: creator, canaryPercent: 0,
    });
    return release.id;
  }

  it('returns 409 when no approvers', async () => {
    const id = seedApprovedRelease('alice', []);
    const response = await app.inject({ method: 'PUT', url: `/api/releases/${id}/activate` });
    expect(response.statusCode).toBe(409);
  });

  it('returns 409 when creator is approver (maker-checker)', async () => {
    const id = seedApprovedRelease('alice', ['alice', 'bob']);
    const response = await app.inject({ method: 'PUT', url: `/api/releases/${id}/activate` });
    expect(response.statusCode).toBe(409);
  });

  it('returns 409 when only 1 approver', async () => {
    const id = seedApprovedRelease('alice', ['bob']);
    const response = await app.inject({ method: 'PUT', url: `/api/releases/${id}/activate` });
    expect(response.statusCode).toBe(409);
  });

  it('returns 200 when 2+ distinct approvers (different from creator)', async () => {
    const id = seedApprovedRelease('alice', ['bob', 'carol']);
    const response = await app.inject({ method: 'PUT', url: `/api/releases/${id}/activate` });
    expect(response.statusCode).toBe(200);
    const body = response.json() as { status: string };
    expect(body.status).toBe('active');
  });

  it('returns 404 for unknown release', async () => {
    const response = await app.inject({ method: 'PUT', url: '/api/releases/nonexistent/activate' });
    expect(response.statusCode).toBe(404);
  });
});