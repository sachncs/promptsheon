import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import Database from 'better-sqlite3';
import { runMigrations } from '../src/db/index.js';
import { ProjectRepo } from '../src/repos/project.js';
import { CapabilityRepo } from '../src/repos/capability.js';
import { CapabilityService } from '../src/application/capability-service.js';
import { registerCapabilityRoutes } from '../src/routes/capability.js';

describe('capability routes', () => {
  let app: FastifyInstance;
  let db: Database.Database;
  let projectId: string;

  beforeEach(async () => {
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    await runMigrations(db);

    const workspaceId = crypto.randomUUID();
    db.prepare(
      `INSERT INTO workspaces (id, name, organization, org_id, created_at, updated_at)
       VALUES (?, 'test', '', 'legacy', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
    ).run(workspaceId);
    projectId = new ProjectRepo(db).create({ workspaceId, name: 'test-project' }).id;

    app = Fastify({ logger: false });
    app.addHook('onRequest', async (request) => {
      (request as unknown as { agentOrgId: string }).agentOrgId = 'legacy';
    });
    registerCapabilityRoutes(app, new CapabilityService(new CapabilityRepo(db)));
    await app.ready();
  });

  afterEach(async () => {
    await app.close();
    db.close();
  });

  it('creates and lists capabilities through the application service', async () => {
    const created = await app.inject({
      method: 'POST',
      url: '/api/capabilities',
      payload: { projectId, name: 'refund-triage' },
    });
    expect(created.statusCode).toBe(201);

    const listed = await app.inject({
      method: 'GET',
      url: `/api/capabilities?projectId=${projectId}`,
    });
    expect(listed.statusCode).toBe(200);
    const items = listed.json<Array<{ name: string }>>();
    expect(items).toHaveLength(1);
    expect(items[0]?.name).toBe('refund-triage');
    expect((items[0] as { selfEvolveEnabled: unknown }).selfEvolveEnabled).toBe(false);
  });

  it('validates ids and returns not found for missing capabilities', async () => {
    const malformed = await app.inject({ method: 'GET', url: '/api/capabilities/not-a-uuid' });
    expect(malformed.statusCode).toBe(422);

    const missing = await app.inject({
      method: 'DELETE',
      url: '/api/capabilities/00000000-0000-0000-0000-000000000000',
    });
    expect(missing.statusCode).toBe(404);
    expect(missing.json<{ error: { code: string } }>().error.code).toBe('NOT_FOUND');
  });
});
