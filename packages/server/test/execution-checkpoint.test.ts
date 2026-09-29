import Database from 'better-sqlite3';
import { beforeEach, describe, expect, it } from 'vitest';
import { applyMigrations, type MigrationSql } from '@promptsheon/shared';
import { ExecutionCheckpointRepo } from '../src/repos/execution-checkpoint.js';
import { ExecutionJobRepo } from '../src/repos/execution-job.js';
import { readFileSync, readdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const migrationsDir = join(dirname(fileURLToPath(import.meta.url)), '..', '..', 'shared', 'db', 'migrations');
const migrations: MigrationSql[] = readdirSync(migrationsDir)
  .filter((file) => file.endsWith('.up.sql'))
  .map((file) => ({ version: Number.parseInt(file.split('_')[0] ?? '0', 10), name: file, up: readFileSync(join(migrationsDir, file), 'utf8') }))
  .sort((left, right) => left.version - right.version);

describe('ExecutionCheckpointRepo', () => {
  let db: Database.Database;
  let repo: ExecutionCheckpointRepo;
  let executionId: string;

  beforeEach(() => {
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, migrations);
    db.prepare("INSERT INTO orgs (id, name, slug, created_at, updated_at) VALUES ('org1', 'One', 'one', '2026-01-01', '2026-01-01')").run();
    db.prepare("INSERT INTO workspaces (id, name, organization, org_id, created_at, updated_at) VALUES ('ws1', 'Workspace', '', 'org1', '2026-01-01', '2026-01-01')").run();
    executionId = new ExecutionJobRepo(db).enqueue({ organizationId: 'org1', workspaceId: 'ws1', agentHash: 'a'.repeat(64), inputHash: 'i'.repeat(64), inputJson: '{}', idempotencyKey: 'checkpoint' }).id;
    repo = new ExecutionCheckpointRepo(db);
  });

  it('upserts steps and lists them in deterministic order', () => {
    repo.save({ executionId, stepId: 'b', state: 'completed', output: 'two', metadata: { index: 2 } });
    repo.save({ executionId, stepId: 'a', state: 'completed', output: 'one', metadata: { index: 1 } });
    repo.save({ executionId, stepId: 'b', state: 'completed', output: 'updated', metadata: { index: 3 } });
    expect(repo.list(executionId).map((checkpoint) => [checkpoint.stepId, checkpoint.output])).toEqual([['a', 'one'], ['b', 'updated']]);
    expect(repo.clear(executionId)).toBe(2);
    expect(repo.list(executionId)).toEqual([]);
  });
});
