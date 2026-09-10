import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import Database from 'better-sqlite3';
import { runMigrations } from '../src/db/index.js';
import { WorkspaceRepo } from '../src/repos/workspace.js';
import { registerWorkspaceRoutes } from '../src/routes/workspace.js';
import { registerHealthRoutes } from '../src/routes/health.js';

describe('Fastify routes', () => {
  let db: Database.Database;
  let app: FastifyInstance;

  beforeEach(async () => {
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    await runMigrations(db);

    app = Fastify({ logger: false });
    const workspaceRepo = new WorkspaceRepo(db);
    registerWorkspaceRoutes(app, workspaceRepo);
    registerHealthRoutes(app, db);
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
});
