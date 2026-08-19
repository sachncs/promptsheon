import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Database from 'better-sqlite3';
import { ManifestRepo, computeManifestHash } from '../src/repos/manifest.js';
import { applyMigrations } from '@promptsheon/shared';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import type { Manifest, MigrationSql } from '@promptsheon/shared';

const __dirname = dirname(fileURLToPath(import.meta.url));
const MIGRATIONS_DIR = join(__dirname, '..', '..', 'shared', 'db', 'migrations');

function loadAllMigrations(): MigrationSql[] {
  const files = readdirSync(MIGRATIONS_DIR).filter((f) => f.endsWith('.up.sql'));
  return files
    .map((f) => {
      const version = parseInt(f.split('_')[0], 10);
      const up = readFileSync(join(MIGRATIONS_DIR, f), 'utf-8');
      return { version, name: f, up };
    })
    .filter((m) => m.version !== 0)
    .sort((a, b) => a.version - b.version);
}

function buildValidManifest(overrides: Partial<Manifest> = {}): Manifest {
  return {
    id: 'm1',
    version: 1,
    prompt: { systemPrompt: 'You are helpful.', userTemplate: '{{input}}' },
    model: { provider: 'openai', modelId: 'gpt-4', temperature: 0.7, maxTokens: 4096 },
    runtime: {
      timeoutMs: 30000,
      nodeTimeoutMs: 10000,
      totalTimeoutMs: 300000,
      maxRetries: 3,
      canaryPercent: 0,
      concurrencyLimit: 10,
    },
    context: { inputsSchema: {}, outputsSchema: {}, requiredContextVars: [] },
    memory: { enabled: false, type: 'stateless' },
    guardrails: { pre: [], post: [] },
    tools: [],
    mcpServers: [],
    evaluation: { datasets: [], scorers: [], passThreshold: 0.7 },
    nodes: [],
    edges: [],
    metadata: {},
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

describe('computeManifestHash', () => {
  it('produces a 64-char hex SHA-256', () => {
    const h = computeManifestHash(buildValidManifest());
    expect(h).toMatch(/^[0-9a-f]{64}$/);
  });

  it('is deterministic for the same content', () => {
    const m1 = buildValidManifest();
    const m2 = buildValidManifest();
    expect(computeManifestHash(m1)).toBe(computeManifestHash(m2));
  });

  it('changes when content changes', () => {
    const m1 = buildValidManifest();
    const m2 = buildValidManifest({ version: 2 });
    expect(computeManifestHash(m1)).not.toBe(computeManifestHash(m2));
  });
});

describe('ManifestRepo', () => {
  let db: ReturnType<typeof Database>;
  let repo: ManifestRepo;

  beforeEach(() => {
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
    db.prepare(`
      INSERT INTO workspaces (id, name, organization, created_at, updated_at)
      VALUES ('ws1', 'Test Workspace', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
    `).run();
    db.prepare(`
      INSERT INTO projects (id, workspace_id, name, description, created_at, updated_at)
      VALUES ('proj1', 'ws1', 'Test Project', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
    `).run();
    db.prepare(`
      INSERT INTO capabilities (id, project_id, name, description, created_at, updated_at)
      VALUES ('cap1', 'proj1', 'Test', 'desc', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')
    `).run();
    repo = new ManifestRepo(db);
  });

  afterEach(() => db.close());

  describe('create + findByHash', () => {
    it('persists a single-node manifest and reads it back', () => {
      const manifest = buildValidManifest({
        nodes: [
          {
            id: 'root',
            name: 'Root',
            description: 'd',
            goal: 'g',
            manifest: buildValidManifest({ id: 'leaf', version: 1 }),
            dependsOn: [],
            preGuardrails: [],
            postGuardrails: [],
            observability: { logInputs: true, logOutputs: true, trackLatency: true, trackCost: true },
            hooks: { beforeInvocation: false, afterInvocation: false, beforeModelCall: false, afterModelCall: false, beforeToolCall: false, afterToolCall: false },
            retry: { kind: 'exponential', maxAttempts: 3, baseDelayMs: 1000, maxDelayMs: 10000 },
            conversationManager: { kind: 'sliding-window', windowSize: 20 },
            state: { enabled: false, type: 'stateless' },
            limits: {},
          },
        ],
        metadata: { capabilityId: 'cap1' },
      });
      const hash = repo.create(manifest, { goal: 'g', createdBy: 'alice' });
      const loaded = repo.findByHash(hash);
      expect(loaded).not.toBeNull();
      expect(loaded!.id).toBe('m1');
      expect(loaded!.nodes.length).toBe(1);
      expect(loaded!.nodes[0].id).toBe('root');
    });

    it('persists edges alongside nodes', () => {
      const leaf = buildValidManifest({ id: 'leaf', version: 1 });
      const manifest = buildValidManifest({
        nodes: [
          { id: 'a', name: 'A', description: '', goal: 'ga', manifest: leaf, dependsOn: [], preGuardrails: [], postGuardrails: [], observability: { logInputs: true, logOutputs: true, trackLatency: true, trackCost: true }, hooks: { beforeInvocation: false, afterInvocation: false, beforeModelCall: false, afterModelCall: false, beforeToolCall: false, afterToolCall: false }, retry: { kind: 'exponential', maxAttempts: 3, baseDelayMs: 1000, maxDelayMs: 10000 }, conversationManager: { kind: 'sliding-window', windowSize: 20 }, state: { enabled: false, type: 'stateless' }, limits: {} },
          { id: 'b', name: 'B', description: '', goal: 'gb', manifest: leaf, dependsOn: ['a'], preGuardrails: [], postGuardrails: [], observability: { logInputs: true, logOutputs: true, trackLatency: true, trackCost: true }, hooks: { beforeInvocation: false, afterInvocation: false, beforeModelCall: false, afterModelCall: false, beforeToolCall: false, afterToolCall: false }, retry: { kind: 'exponential', maxAttempts: 3, baseDelayMs: 1000, maxDelayMs: 10000 }, conversationManager: { kind: 'sliding-window', windowSize: 20 }, state: { enabled: false, type: 'stateless' }, limits: {} },
        ],
        edges: [{ from: 'a', to: 'b', mapping: { x: 'y' } }],
        metadata: { capabilityId: 'cap1' },
      });
      const hash = repo.create(manifest, { goal: 'g', createdBy: 'alice' });
      const loaded = repo.findByHash(hash)!;
      expect(loaded.edges).toHaveLength(1);
      expect(loaded.edges[0]).toEqual({ from: 'a', to: 'b', mapping: { x: 'y' } });
    });

    it('is idempotent: same manifest → same hash', () => {
      const manifest = buildValidManifest({ metadata: { capabilityId: 'cap1' } });
      const hash1 = repo.create(manifest, { goal: 'g', createdBy: 'alice' });
      expect(() => repo.create(manifest, { goal: 'g', createdBy: 'alice' })).toThrow(/UNIQUE/);
      expect(repo.findByHash(hash1)).not.toBeNull();
    });
  });

  describe('findByCapabilityAndVersion', () => {
    it('returns manifest by composite key', () => {
      const manifest = buildValidManifest({ version: 3, metadata: { capabilityId: 'cap1' } });
      const hash = repo.create(manifest, { goal: 'g', createdBy: 'alice' });
      const loaded = repo.findByCapabilityAndVersion('cap1', 3);
      expect(loaded).not.toBeNull();
      expect(loaded!.id).toBe('m1');
    });

    it('returns null for missing version', () => {
      expect(repo.findByCapabilityAndVersion('cap1', 99)).toBeNull();
    });
  });

  describe('findLatestByCapability', () => {
    it('returns highest-version manifest', () => {
      repo.create(buildValidManifest({ version: 1, metadata: { capabilityId: 'cap1' } }), { goal: 'g', createdBy: 'alice' });
      repo.create(buildValidManifest({ version: 2, metadata: { capabilityId: 'cap1' } }), { goal: 'g', createdBy: 'alice' });
      repo.create(buildValidManifest({ version: 5, metadata: { capabilityId: 'cap1' } }), { goal: 'g', createdBy: 'alice' });
      const loaded = repo.findLatestByCapability('cap1')!;
      expect(loaded.version).toBe(5);
    });
  });

  describe('approvals', () => {
    it('records approval votes', () => {
      const manifest = buildValidManifest({ metadata: { capabilityId: 'cap1' } });
      const hash = repo.create(manifest, { goal: 'g', createdBy: 'alice' });
      repo.upsertApproval(hash, 'user1', 'approve');
      repo.upsertApproval(hash, 'user2', 'approve');
      const approvals = repo.findApprovals(hash);
      expect(approvals).toHaveLength(2);
      expect(repo.countDistinctApprovers(hash)).toBe(2);
    });

    it('overwrites prior vote on re-vote', () => {
      const manifest = buildValidManifest({ metadata: { capabilityId: 'cap1' } });
      const hash = repo.create(manifest, { goal: 'g', createdBy: 'alice' });
      repo.upsertApproval(hash, 'user1', 'approve');
      repo.upsertApproval(hash, 'user1', 'reject');
      expect(repo.countDistinctApprovers(hash)).toBe(0);
      expect(repo.findApprovals(hash)[0].vote).toBe('reject');
    });

    it('throws on missing manifest hash', () => {
      expect(() => repo.upsertApproval('nonexistent', 'user1', 'approve')).toThrow();
    });
  });

  describe('nodeRuns', () => {
    it('records and reads back node runs', () => {
      const manifest = buildValidManifest({ metadata: { capabilityId: 'cap1' } });
      const hash = repo.create(manifest, { goal: 'g', createdBy: 'alice' });
      repo.recordNodeRun({
        manifestHash: hash,
        nodeId: 'root',
        executionId: 'exec1',
        startedAt: '2026-01-01T00:00:00Z',
        endedAt: '2026-01-01T00:01:00Z',
        latencyMs: 60000,
        costUsd: 0.05,
        totalTokens: 100,
        status: 'completed',
      });
      const runs = repo.findNodeRunsByExecution('exec1');
      expect(runs).toHaveLength(1);
      expect(runs[0].manifestHash).toBe(hash);
      expect(runs[0].nodeId).toBe('root');
      expect(runs[0].costUsd).toBe(0.05);
    });
  });

  describe('ensureCutover (hard cutover migration)', () => {
    function insertLegacyVersion(id: string, capabilityId: string, version: number, systemPrompt: string): void {
      db.prepare(`
        INSERT INTO capability_versions (id, capability_id, version, manifest, manifest_hash, created_by, created_at)
        VALUES (?, ?, ?, ?, '', 'legacy', '2026-01-01T00:00:00Z')
      `).run(id, capabilityId, version, JSON.stringify({ systemPrompt }));
    }

    it('migrates rows with empty manifest_hash', () => {
      insertLegacyVersion('cv1', 'cap1', 1, 'You are helpful');
      insertLegacyVersion('cv2', 'cap1', 2, 'You are smart');

      const report = repo.ensureCutover({ createdBy: 'cutover' });
      expect(report.scanned).toBe(2);
      expect(report.migrated).toBe(2);
      expect(report.skipped).toBe(0);
      expect(report.errors).toHaveLength(0);

      const v1 = db.prepare("SELECT manifest_hash FROM capability_versions WHERE id = 'cv1'").get() as { manifest_hash: string };
      const v2 = db.prepare("SELECT manifest_hash FROM capability_versions WHERE id = 'cv2'").get() as { manifest_hash: string };
      expect(v1.manifest_hash).toMatch(/^[0-9a-f]{64}$/);
      expect(v2.manifest_hash).toMatch(/^[0-9a-f]{64}$/);
    });

    it('is idempotent: second run finds nothing to migrate', () => {
      insertLegacyVersion('cv1', 'cap1', 1, 'x');
      repo.ensureCutover({ createdBy: 'cutover' });
      const second = repo.ensureCutover({ createdBy: 'cutover' });
      expect(second.scanned).toBe(0);
      expect(second.migrated).toBe(0);
    });

    it('skips rows that already have manifest_hash', () => {
      db.prepare(`
        INSERT INTO capability_versions (id, capability_id, version, manifest, manifest_hash, created_by, created_at)
        VALUES ('cv1', 'cap1', 1, '{}', 'existing-hash', 'legacy', '2026-01-01T00:00:00Z')
      `).run();
      const report = repo.ensureCutover({ createdBy: 'cutover' });
      expect(report.scanned).toBe(0);
      expect(report.migrated).toBe(0);
    });

    it('handles malformed JSON gracefully', () => {
      db.prepare(`
        INSERT INTO capability_versions (id, capability_id, version, manifest, manifest_hash, created_by, created_at)
        VALUES ('cv1', 'cap1', 1, 'NOT-VALID-JSON', '', 'legacy', '2026-01-01T00:00:00Z')
      `).run();
      const report = repo.ensureCutover({ createdBy: 'cutover' });
      expect(report.scanned).toBe(1);
      expect(report.migrated).toBe(1);
    });
  });
});