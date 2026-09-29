import Fastify from 'fastify';
import Database from 'better-sqlite3';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { applyMigrations, type MigrationSql } from '@promptsheon/shared';
import { ExecutionJobRepo } from '../src/repos/execution-job.js';
import { DurableExecutionService } from '../src/application/durable-execution-service.js';
import { registerExecutionJobRoutes } from '../src/routes/execution-jobs.js';
import { WorkspaceRepo } from '../src/repos/workspace.js';
import { readFileSync, readdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const migrationsDir = join(dirname(fileURLToPath(import.meta.url)), '..', '..', 'shared', 'db', 'migrations');
const migrations: MigrationSql[] = readdirSync(migrationsDir)
  .filter((file) => file.endsWith('.up.sql'))
  .map((file) => ({ version: Number.parseInt(file.split('_')[0] ?? '0', 10), name: file, up: readFileSync(join(migrationsDir, file), 'utf8') }))
  .sort((left, right) => left.version - right.version);

describe('execution job routes', () => {
  let db: Database.Database;
  let app: ReturnType<typeof Fastify>;

  beforeEach(async () => {
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, migrations);
    db.prepare("INSERT INTO orgs (id, name, slug, created_at, updated_at) VALUES ('org1', 'One', 'one', '2026-01-01', '2026-01-01')").run();
    db.prepare("INSERT INTO workspaces (id, name, organization, org_id, created_at, updated_at) VALUES ('11111111-1111-4111-8111-111111111111', 'Workspace', '', 'org1', '2026-01-01', '2026-01-01')").run();
    const service = new DurableExecutionService(
      new ExecutionJobRepo(db),
      { get: async () => { throw new Error('not used'); } },
      { execute: async () => ({}) },
    );
    app = Fastify();
    app.addHook('preHandler', async (request) => {
      request.orgContext = { userId: 'user1', orgId: 'org1', role: 'admin' };
    });
    registerExecutionJobRoutes(app, { service, workspaceRepo: new WorkspaceRepo(db) });
    await app.ready();
  });

  afterEach(async () => {
    await app.close();
    db.close();
  });

  it('enqueues and returns the same job for a duplicate request', async () => {
    const payload = {
      agentHash: 'a'.repeat(64),
      inputs: { question: 'hello' },
      idempotencyKey: 'request-1',
    };
    const url = '/api/workspaces/11111111-1111-4111-8111-111111111111/execution-jobs';
    const first = await app.inject({ method: 'POST', url, payload });
    const second = await app.inject({ method: 'POST', url, payload });
    expect(first.statusCode).toBe(202);
    expect(second.statusCode).toBe(202);
    expect(first.json().id).toBe(second.json().id);
    const fetched = await app.inject({ method: 'GET', url: `${url}/${first.json().id}` });
    expect(fetched.statusCode).toBe(200);
    expect(fetched.json().state).toBe('queued');
  });

  it('rejects missing organization context', async () => {
    const unauthenticated = Fastify();
    const service = new DurableExecutionService(new ExecutionJobRepo(db), { get: async () => { throw new Error('should not get'); } }, { execute: async () => ({}) });
    registerExecutionJobRoutes(unauthenticated, { service, workspaceRepo: new WorkspaceRepo(db) });
    const response = await unauthenticated.inject({ method: 'GET', url: `/api/workspaces/11111111-1111-4111-8111-111111111111/execution-jobs/${crypto.randomUUID()}` });
    expect(response.statusCode).toBe(401);
    await unauthenticated.close();
  });

  it('returns a conflict for an idempotency key with different content', async () => {
    const url = '/api/workspaces/11111111-1111-4111-8111-111111111111/execution-jobs';
    const first = await app.inject({ method: 'POST', url, payload: { agentHash: 'a'.repeat(64), inputs: {}, idempotencyKey: 'conflict' } });
    const second = await app.inject({ method: 'POST', url, payload: { agentHash: 'b'.repeat(64), inputs: {}, idempotencyKey: 'conflict' } });
    expect(first.statusCode).toBe(202);
    expect(second.statusCode).toBe(409);
    expect(second.json().error.code).toBe('IDEMPOTENCY_CONFLICT');
  });

  it('does not expose a job through a different workspace path', async () => {
    const create = await app.inject({
      method: 'POST',
      url: '/api/workspaces/11111111-1111-4111-8111-111111111111/execution-jobs',
      payload: { agentHash: 'a'.repeat(64), inputs: {}, idempotencyKey: 'workspace-scope' },
    });
    const jobId = create.json().id as string;
    const response = await app.inject({
      method: 'GET',
      url: `/api/workspaces/22222222-2222-4222-8222-222222222222/execution-jobs/${jobId}`,
    });
    expect(response.statusCode).toBe(404);
  });
});
