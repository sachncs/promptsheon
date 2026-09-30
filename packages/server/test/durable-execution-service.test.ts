import Database from 'better-sqlite3';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { AgentSpecificationSchema, applyMigrations, type MigrationSql } from '@promptsheon/shared';
import { DurableExecutionService } from '../src/application/durable-execution-service.js';
import { ExecutionCheckpointRepo } from '../src/repos/execution-checkpoint.js';
import { ExecutionJobRepo } from '../src/repos/execution-job.js';
import { AsyncEvidenceSink } from '../src/observability/evidence-sink.js';
import type { AppendEvidenceInput } from '../src/repos/evidence.js';
import { readFileSync, readdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const migrationsDir = join(dirname(fileURLToPath(import.meta.url)), '..', '..', 'shared', 'db', 'migrations');
const migrations: MigrationSql[] = readdirSync(migrationsDir)
  .filter((file) => file.endsWith('.up.sql'))
  .map((file) => ({ version: Number.parseInt(file.split('_')[0] ?? '0', 10), name: file, up: readFileSync(join(migrationsDir, file), 'utf8') }))
  .sort((left, right) => left.version - right.version);

async function waitFor(check: () => boolean): Promise<void> {
  for (let attempt = 0; attempt < 100; attempt++) {
    if (check()) return;
    await new Promise((resolve) => setTimeout(resolve, 5));
  }
  throw new Error('condition was not reached');
}

describe('DurableExecutionService', () => {
  let db: Database.Database;

  beforeEach(() => {
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, migrations);
    db.prepare("INSERT INTO orgs (id, name, slug, created_at, updated_at) VALUES ('org1', 'One', 'one', '2026-01-01', '2026-01-01')").run();
    db.prepare("INSERT INTO workspaces (id, name, organization, org_id, created_at, updated_at) VALUES ('ws1', 'Workspace', '', 'org1', '2026-01-01', '2026-01-01')").run();
  });

  afterEach(() => db.close());

  it('converts a stored agent specification into an executable manifest', async () => {
    const agentHash = 'a'.repeat(64);
    const specification = AgentSpecificationSchema.parse({
      role: 'Simulator assistant',
      objective: 'Answer the supplied input.',
      prompt: { system: 'Be concise.' },
      modelPolicy: { provider: 'simulator', model: 'promptsheon-simulator' },
      lifecycle: { owner: 'test-team' },
    });
    let executedManifest: { nodes: Array<{ name: string }>; model: { provider: string; modelId: string }; metadata: Record<string, unknown> } | undefined;
    const service = new DurableExecutionService(
      new ExecutionJobRepo(db),
      { get: async () => ({ hash: agentHash, workspaceId: 'ws1', schemaVersion: '1.0', parentHash: null, author: 'test', changeReason: 'test', status: 'published' as const, createdAt: '2026-01-01', publishedAt: '2026-01-01', specification }) },
      {
        execute: async (_hash, manifest) => {
          executedManifest = manifest;
          return { totalTokens: 2, totalCost: 0, totalLatencyMs: 1 };
        },
      },
      new ExecutionCheckpointRepo(db),
    );
    const worker = service.createWorker({ workerId: 'service-test', pollMs: 2, leaseMs: 500, maxBackoffMs: 1, random: () => 0 });
    const job = service.enqueue({ organizationId: 'org1', workspaceId: 'ws1', agentHash, inputs: { input: 'hello' }, idempotencyKey: 'service-test' });
    worker.start();
    await waitFor(() => new ExecutionJobRepo(db).get('org1', job.id).state === 'completed');
    await worker.stop();

    expect(executedManifest).toMatchObject({
      model: { provider: 'simulator', modelId: 'promptsheon-simulator' },
      metadata: { agentHash, workspaceId: 'ws1' },
    });
    expect(executedManifest?.nodes[0]?.name).toBe('Simulator assistant');
  });

  it('flushes execution evidence before marking a job completed', async () => {
    const written: AppendEvidenceInput[] = [];
    const evidence = new AsyncEvidenceSink({ append: (record) => written.push(record) });
    const agentHash = 'b'.repeat(64);
    const specification = AgentSpecificationSchema.parse({
      role: 'Evidence assistant',
      objective: 'Record completion evidence.',
      prompt: { system: 'Be concise.' },
      modelPolicy: { provider: 'simulator', model: 'promptsheon-simulator' },
      lifecycle: { owner: 'test-team' },
    });
    const service = new DurableExecutionService(
      new ExecutionJobRepo(db),
      { get: async () => ({ hash: agentHash, workspaceId: 'ws1', schemaVersion: '1.0', parentHash: null, author: 'test', changeReason: 'test', status: 'published' as const, createdAt: '2026-01-01', publishedAt: '2026-01-01', specification }) },
      { execute: async () => ({ totalTokens: 1, totalCost: 0, totalLatencyMs: 1 }) },
      new ExecutionCheckpointRepo(db),
      undefined,
      undefined,
      evidence,
    );
    const worker = service.createWorker({ workerId: 'evidence-test', pollMs: 2, leaseMs: 500, maxBackoffMs: 1, random: () => 0 });
    const job = service.enqueue({ organizationId: 'org1', workspaceId: 'ws1', agentHash, inputs: {}, idempotencyKey: 'evidence-test' });
    worker.start();
    await waitFor(() => new ExecutionJobRepo(db).get('org1', job.id).state === 'completed');
    await worker.stop();

    expect(written.map((record) => record.eventType)).toEqual([
      'execution.started',
      'execution.completed',
      'resource.consumed',
    ]);
  });
});
