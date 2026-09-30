import { beforeEach, describe, expect, it } from 'vitest';
import Database from 'better-sqlite3';
import { applyMigrations, type MigrationSql } from '@promptsheon/shared';
import { ExecutionJobRepo, ExecutionQueueCapacityError } from '../src/repos/execution-job.js';
import { readFileSync, readdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const migrationsDir = join(dirname(fileURLToPath(import.meta.url)), '..', '..', 'shared', 'db', 'migrations');
const migrations: MigrationSql[] = readdirSync(migrationsDir)
  .filter((file) => file.endsWith('.up.sql'))
  .map((file) => ({
    version: Number.parseInt(file.split('_')[0] ?? '0', 10),
    name: file,
    up: readFileSync(join(migrationsDir, file), 'utf8'),
  }))
  .sort((left, right) => left.version - right.version);

describe('ExecutionJobRepo', () => {
  let db: Database.Database;
  let repo: ExecutionJobRepo;

  beforeEach(() => {
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, migrations);
    db.prepare("INSERT INTO orgs (id, name, slug, created_at, updated_at) VALUES ('org1', 'One', 'one', '2026-01-01', '2026-01-01')").run();
    db.prepare("INSERT INTO workspaces (id, name, organization, org_id, created_at, updated_at) VALUES ('ws1', 'Workspace', '', 'org1', '2026-01-01', '2026-01-01')").run();
    repo = new ExecutionJobRepo(db);
  });

  it('deduplicates an idempotency key and rejects a conflicting request', () => {
    const first = repo.enqueue({ organizationId: 'org1', workspaceId: 'ws1', agentHash: 'a'.repeat(64), inputHash: 'i'.repeat(64), inputJson: '{}', idempotencyKey: 'request-1' });
    const duplicate = repo.enqueue({ organizationId: 'org1', workspaceId: 'ws1', agentHash: 'a'.repeat(64), inputHash: 'i'.repeat(64), inputJson: '{}', idempotencyKey: 'request-1' });
    expect(duplicate.id).toBe(first.id);
    expect(() => repo.enqueue({ organizationId: 'org1', workspaceId: 'ws1', agentHash: 'b'.repeat(64), inputHash: 'i'.repeat(64), inputJson: '{}', idempotencyKey: 'request-1' })).toThrow(/different execution request/);
  });

  it('claims, transitions, and enforces lifecycle rules', () => {
    const queued = repo.enqueue({ organizationId: 'org1', workspaceId: 'ws1', agentHash: 'a'.repeat(64), inputHash: 'i'.repeat(64), inputJson: '{"x":1}', idempotencyKey: 'request-2' });
    const running = repo.claimNext('worker-1', 10_000);
    expect(running?.id).toBe(queued.id);
    expect(running?.state).toBe('running');
    expect(running?.attempts).toBe(1);
    const completed = repo.transition('org1', queued.id, 'running', 'completed', { resultJson: '{"ok":true}' });
    expect(completed.resultJson).toBe('{"ok":true}');
    expect(() => repo.transition('org1', queued.id, 'completed', 'running')).toThrow(/invalid execution transition/);
    expect(repo.cancel('org1', queued.id).state).toBe('completed');
  });

  it('requeues expired leases and fails exhausted jobs', () => {
    const retryable = repo.enqueue({ organizationId: 'org1', workspaceId: 'ws1', agentHash: 'a'.repeat(64), inputHash: 'i'.repeat(64), inputJson: '{}', idempotencyKey: 'request-3', maxAttempts: 2 });
    const exhausted = repo.enqueue({ organizationId: 'org1', workspaceId: 'ws1', agentHash: 'b'.repeat(64), inputHash: 'j'.repeat(64), inputJson: '{}', idempotencyKey: 'request-4', maxAttempts: 1 });
    const first = repo.claimNext('worker-1', 1);
    const second = repo.claimNext('worker-1', 1);
    expect(first?.id).toBe(retryable.id);
    expect(second?.id).toBe(exhausted.id);
    db.prepare("UPDATE execution_jobs SET lease_expires_at = '2020-01-01T00:00:00.000Z'").run();
    expect(repo.requeueExpired(new Date('2026-01-01T00:00:00.000Z'))).toBe(1);
    expect(repo.get('org1', retryable.id).state).toBe('queued');
    expect(repo.get('org1', exhausted.id).state).toBe('failed');
  });

  it('reports queue depth and age by organization', () => {
    repo.enqueue({ organizationId: 'org1', workspaceId: 'ws1', agentHash: 'a'.repeat(64), inputHash: 'i'.repeat(64), inputJson: '{}', idempotencyKey: 'metrics' });
    expect(repo.metrics('org1')).toMatchObject({ queued: 1, running: 0, completed: 0, oldestQueuedAt: expect.any(String) });
  });

  it('rejects new work at the organization queue limit but preserves idempotent retries', () => {
    const limited = new ExecutionJobRepo(db, 1);
    const first = limited.enqueue({ organizationId: 'org1', workspaceId: 'ws1', agentHash: 'a'.repeat(64), inputHash: 'i'.repeat(64), inputJson: '{}', idempotencyKey: 'capacity-1' });
    expect(() => limited.enqueue({ organizationId: 'org1', workspaceId: 'ws1', agentHash: 'b'.repeat(64), inputHash: 'j'.repeat(64), inputJson: '{}', idempotencyKey: 'capacity-2' })).toThrow(ExecutionQueueCapacityError);
    expect(limited.enqueue({ organizationId: 'org1', workspaceId: 'ws1', agentHash: 'a'.repeat(64), inputHash: 'i'.repeat(64), inputJson: '{}', idempotencyKey: 'capacity-1' }).id).toBe(first.id);
  });

  it('rolls back the inserted job when an admission callback rejects it', () => {
    expect(() => repo.enqueue({
      organizationId: 'org1', workspaceId: 'ws1', agentHash: 'a'.repeat(64), inputHash: 'i'.repeat(64), inputJson: '{}', idempotencyKey: 'admission-rejected',
      afterInsert: () => { throw new Error('quota exceeded'); },
    })).toThrow('quota exceeded');
    expect(repo.findByIdempotency('org1', 'admission-rejected')).toBeNull();
    expect(repo.metrics('org1').queued).toBe(0);
  });

  it('claims a high-volume queue exactly once', () => {
    const jobs = Array.from({ length: 250 }, (_, index) => repo.enqueue({
      organizationId: 'org1', workspaceId: 'ws1', agentHash: 'a'.repeat(64), inputHash: String(index).padStart(64, '0'), inputJson: JSON.stringify({ index }), idempotencyKey: `load-${index}`,
    }));
    const claimed = new Set<string>();
    for (let index = 0; index < jobs.length; index++) {
      const job = repo.claimNext('load-worker', 10_000);
      expect(job).not.toBeNull();
      if (job) claimed.add(job.id);
    }
    expect(claimed.size).toBe(jobs.length);
    expect(repo.claimNext('load-worker', 10_000)).toBeNull();
  });
});
