import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import Database from 'better-sqlite3';
import { runMigrations } from '../src/db/index.js';
import { WorkspaceRepo } from '../src/repos/workspace.js';
import { WorkspaceService } from '../src/application/workspace-service.js';
import { registerWorkspaceRoutes } from '../src/routes/workspace.js';
import { registerHealthRoutes } from '../src/routes/health.js';
import { HealthService } from '../src/application/health-service.js';
import { SqliteHealthProbe } from '../src/infrastructure/sqlite-health-probe.js';

describe('Fastify routes', () => {
  let db: Database.Database;
  let app: FastifyInstance;

  beforeEach(async () => {
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    await runMigrations(db);

    app = Fastify({ logger: false });
    app.addHook('onRequest', async (request) => {
      (request as unknown as { agentOrgId: string }).agentOrgId = 'legacy';
    });
    const workspaceRepo = new WorkspaceRepo(db);
    registerWorkspaceRoutes(app, new WorkspaceService(workspaceRepo));
    registerHealthRoutes(app, new HealthService(new SqliteHealthProbe(db)));
    await app.ready();
  });

  afterEach(async () => {
    await app.close();
    db.close();
  });

  it('GET /api/health returns 200', async () => {
    const res = await app.inject({ method: 'GET', url: '/api/health' });
    expect(res.statusCode).toBe(200);
    const body = res.json() as { status: string; db: string; timestamp: string };
    expect(body.status).toBe('ok');
    expect(body.db).toBe('ok');
    expect(typeof body.timestamp).toBe('string');
  });

  it('GET /api/ready verifies database readiness', async () => {
    const res = await app.inject({ method: 'GET', url: '/api/ready' });
    expect(res.statusCode).toBe(200);
    expect(res.json<{ status: string; db: string }>()).toMatchObject({ status: 'ready', db: 'ok' });
  });

  it('GET /api/health fails closed when the database is unhealthy', async () => {
    const unhealthyApp = Fastify({ logger: false });
    registerHealthRoutes(
      unhealthyApp,
      new HealthService({ ping: () => false, quickCheck: () => false }),
    );
    await unhealthyApp.ready();

    const res = await unhealthyApp.inject({ method: 'GET', url: '/api/health' });

    expect(res.statusCode).toBe(503);
    expect(res.json<{ status: string; db: string; error: string }>()).toMatchObject({
      status: 'error',
      db: 'error',
      error: 'database unavailable',
    });
    await unhealthyApp.close();
  });

  it('GET /api/ready fails closed when the database is not ready', async () => {
    const notReadyApp = Fastify({ logger: false });
    registerHealthRoutes(
      notReadyApp,
      new HealthService({ ping: () => true, quickCheck: () => false }),
    );
    await notReadyApp.ready();

    const res = await notReadyApp.inject({ method: 'GET', url: '/api/ready' });

    expect(res.statusCode).toBe(503);
    expect(res.json<{ status: string; db: string }>()).toMatchObject({
      status: 'not_ready',
      db: 'error',
    });
    await notReadyApp.close();
  });

  it('POST /api/workspaces then GET /api/workspaces/:id returns 200', async () => {
    const createRes = await app.inject({
      method: 'POST',
      url: '/api/workspaces',
      payload: { name: 'acme', organization: 'ACME Inc' },
    });
    expect(createRes.statusCode).toBe(201);
    const created = createRes.json() as { id: string; name: string; organization: string };
    expect(created.id).toBeTypeOf('string');
    expect(created.name).toBe('acme');

    const getRes = await app.inject({ method: 'GET', url: `/api/workspaces/${created.id}` });
    expect(getRes.statusCode).toBe(200);
    const fetched = getRes.json() as { id: string; name: string; organization: string };
    expect(fetched.id).toBe(created.id);
    expect(fetched.name).toBe('acme');
    expect(fetched.organization).toBe('ACME Inc');
  });

  it('POST /api/workspaces with invalid body returns 422', async () => {
    const res = await app.inject({
      method: 'POST',
      url: '/api/workspaces',
      payload: { name: '' },
    });
    expect(res.statusCode).toBe(422);
    const body = res.json() as { error: { code: string; issues: unknown[] } };
    expect(body.error.code).toBe('VALIDATION_ERROR');
    expect(Array.isArray(body.error.issues)).toBe(true);
  });

  it('GET /api/workspaces/:id with unknown id returns 404', async () => {
    const res = await app.inject({
      method: 'GET',
      url: '/api/workspaces/00000000-0000-0000-0000-000000000000',
    });
    expect(res.statusCode).toBe(404);
  });

  it('rejects malformed workspace ids before querying the repository', async () => {
    const res = await app.inject({ method: 'GET', url: '/api/workspaces/not-a-uuid' });
    expect(res.statusCode).toBe(422);
    expect(res.json<{ error: { code: string } }>().error.code).toBe('VALIDATION_ERROR');
  });

  it('returns 404 when deleting a workspace that does not exist', async () => {
    const res = await app.inject({
      method: 'DELETE',
      url: '/api/workspaces/00000000-0000-0000-0000-000000000000',
    });
    expect(res.statusCode).toBe(404);
    expect(res.json<{ error: { code: string } }>().error.code).toBe('NOT_FOUND');
  });
});
