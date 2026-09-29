import { beforeEach, describe, expect, it } from 'vitest';
import Database from 'better-sqlite3';
import { applyMigrations, type MigrationSql } from '@promptsheon/shared';
import { ExecutionJobRepo } from '../src/repos/execution-job.js';
import { DurableExecutionWorker, ExecutionWorkError } from '../src/application/durable-execution-worker.js';
import { readFileSync, readdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const migrationsDir = join(dirname(fileURLToPath(import.meta.url)), '..', '..', 'shared', 'db', 'migrations');
const migrations: MigrationSql[] = readdirSync(migrationsDir)
  .filter((file) => file.endsWith('.up.sql'))
  .map((file) => ({ version: Number.parseInt(file.split('_')[0] ?? '0', 10), name: file, up: readFileSync(join(migrationsDir, file), 'utf8') }))
  .sort((left, right) => left.version - right.version);

const hash = (value: string): string => value.repeat(64).slice(0, 64);

async function waitFor(check: () => boolean): Promise<void> {
  for (let attempt = 0; attempt < 100; attempt++) {
    if (check()) return;
    await new Promise((resolve) => setTimeout(resolve, 5));
  }
  throw new Error('condition was not reached');
}

describe('DurableExecutionWorker', () => {
  let db: Database.Database;
  let jobs: ExecutionJobRepo;

  beforeEach(() => {
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, migrations);
    db.prepare("INSERT INTO orgs (id, name, slug, created_at, updated_at) VALUES ('org1', 'One', 'one', '2026-01-01', '2026-01-01')").run();
    db.prepare("INSERT INTO workspaces (id, name, organization, org_id, created_at, updated_at) VALUES ('ws1', 'Workspace', '', 'org1', '2026-01-01', '2026-01-01')").run();
    jobs = new ExecutionJobRepo(db);
  });

  it('enforces bounded concurrency', async () => {
    const queued = ['1', '2', '3', '4'].map((value) => jobs.enqueue({ organizationId: 'org1', workspaceId: 'ws1', agentHash: hash('a'), inputHash: hash(value), inputJson: '{}', idempotencyKey: value }));
    let active = 0;
    let maximum = 0;
    const worker = new DurableExecutionWorker(jobs, {
      async run() {
        active++;
        maximum = Math.max(maximum, active);
        await new Promise((resolve) => setTimeout(resolve, 15));
        active--;
        return { ok: true };
      },
    }, { workerId: 'worker-1', maxConcurrency: 2, pollMs: 2, leaseMs: 500, maxBackoffMs: 10, random: () => 0 });
    worker.start();
    await waitFor(() => queued.every((job) => jobs.get('org1', job.id).state === 'completed'));
    await worker.stop();
    expect(maximum).toBe(2);
  });

  it('retries classified failures with bounded attempts', async () => {
    const job = jobs.enqueue({ organizationId: 'org1', workspaceId: 'ws1', agentHash: hash('a'), inputHash: hash('retry'), inputJson: '{}', idempotencyKey: 'retry', maxAttempts: 2 });
    let calls = 0;
    const worker = new DurableExecutionWorker(jobs, {
      async run() {
        calls++;
        if (calls === 1) throw new ExecutionWorkError('provider unavailable', true);
        return { recovered: true };
      },
    }, { workerId: 'worker-1', maxConcurrency: 1, pollMs: 2, leaseMs: 500, maxBackoffMs: 1, random: () => 0 });
    worker.start();
    await waitFor(() => jobs.get('org1', job.id).state === 'completed');
    await worker.stop();
    expect(calls).toBe(2);
    expect(jobs.get('org1', job.id).attempts).toBe(2);
  });

  it('propagates cancellation to active work', async () => {
    const job = jobs.enqueue({ organizationId: 'org1', workspaceId: 'ws1', agentHash: hash('a'), inputHash: hash('cancel'), inputJson: '{}', idempotencyKey: 'cancel' });
    const worker = new DurableExecutionWorker(jobs, {
      run(_job, context) {
        return new Promise((_, reject) => context.signal.addEventListener('abort', () => reject(new Error('aborted')), { once: true }));
      },
    }, { workerId: 'worker-1', maxConcurrency: 1, pollMs: 2, leaseMs: 500, maxBackoffMs: 1, random: () => 0 });
    worker.start();
    await waitFor(() => jobs.get('org1', job.id).state === 'running');
    worker.cancel('org1', job.id);
    await waitFor(() => jobs.get('org1', job.id).state === 'cancelled');
    await worker.stop();
    expect(jobs.get('org1', job.id).state).toBe('cancelled');
  });
});
