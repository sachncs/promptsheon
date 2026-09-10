import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Fastify, { type FastifyInstance } from 'fastify';
import Database from 'better-sqlite3';
import { applyMigrations } from '@promptsheon/shared';
import { registerBudgetRoutes } from '../src/routes/budget.js';
import { CostBudgetRepo } from '../src/repos/budget.js';
import { CostForecastService } from '../src/analysis/forecast.js';
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
      return { version, name: f, up };
    })
    .filter((m) => m.version !== 0)
    .sort((a, b) => a.version - b.version);
}

function bootstrap(db: Database.Database): void {
  db.prepare(
    `INSERT INTO orgs (id, name, slug, residency, created_at, updated_at) VALUES ('org-1', 'Test', 'test', 'local', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
  ).run();
}

describe('GET /api/admin/budgets + /api/admin/cost-forecast', () => {
  let app: FastifyInstance;
  let db: Database.Database;
  let budgetRepo: CostBudgetRepo;

  beforeEach(async () => {
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
    bootstrap(db);
    budgetRepo = new CostBudgetRepo(db);
    const forecastService = new CostForecastService(db, {
      listBudgets: (orgId) => budgetRepo.listForOrg(orgId),
      updateLastAlerted: (id, ts) => budgetRepo.updateLastAlerted(id, ts),
      persistSnapshot: (snap) => budgetRepo.insertForecastSnapshot(snap),
    });
    app = Fastify();
    app.setErrorHandler((error, _request, reply) => {
      if (error.name === 'NotFoundError') {
        return reply.code(404).send({ error: { code: 'NOT_FOUND', message: error.message } });
      }
      if (error.statusCode) {
        return reply.code(error.statusCode).send({ error: { code: 'APP_ERROR', message: error.message } });
      }
      return reply.code(500).send({ error: { code: 'INTERNAL_ERROR', message: error.message } });
    });
    await app.register(async (instance) => {
      registerBudgetRoutes(instance, { budgetRepo, forecastService });
    });
    await app.ready();
  });

  afterEach(async () => {
    await app.close();
    db.close();
  });

  it('GET /api/admin/budgets returns [] when none exist', async () => {
    const res = await app.inject({ method: 'GET', url: '/api/admin/budgets?organizationId=org-1' });
    expect(res.statusCode).toBe(200);
    expect(res.json()).toEqual({ items: [] });
  });

  it('GET /api/admin/budgets without organizationId returns 400', async () => {
    const res = await app.inject({ method: 'GET', url: '/api/admin/budgets' });
    expect(res.statusCode).toBe(400);
  });

  it('POST /api/admin/budgets creates a budget', async () => {
    const res = await app.inject({
      method: 'POST',
      url: '/api/admin/budgets',
      payload: { organizationId: 'org-1', label: 'monthly', limitMicros: 1_000_000 },
    });
    expect(res.statusCode).toBe(201);
    const body = res.json() as { id: string; label: string };
    expect(body.label).toBe('monthly');
    expect(typeof body.id).toBe('string');
  });

  it('POST with a duplicate label returns 409', async () => {
    await app.inject({
      method: 'POST',
      url: '/api/admin/budgets',
      payload: { organizationId: 'org-1', label: 'dup', limitMicros: 1000 },
    });
    const res = await app.inject({
      method: 'POST',
      url: '/api/admin/budgets',
      payload: { organizationId: 'org-1', label: 'dup', limitMicros: 1000 },
    });
    expect(res.statusCode).toBe(409);
  });

  it('PATCH updates the limit', async () => {
    const created = await app.inject({
      method: 'POST',
      url: '/api/admin/budgets',
      payload: { organizationId: 'org-1', label: 'l', limitMicros: 1000 },
    });
    const id = (created.json() as { id: string }).id;
    const res = await app.inject({
      method: 'PATCH',
      url: `/api/admin/budgets/${id}`,
      payload: { limitMicros: 2000 },
    });
    expect(res.statusCode).toBe(200);
    expect((res.json() as { limitMicros: number }).limitMicros).toBe(2000);
  });

  it('PATCH on unknown id returns 404', async () => {
    const res = await app.inject({
      method: 'PATCH',
      url: '/api/admin/budgets/missing',
      payload: { limitMicros: 1 },
    });
    expect(res.statusCode).toBe(404);
  });

  it('DELETE removes the budget', async () => {
    const created = await app.inject({
      method: 'POST',
      url: '/api/admin/budgets',
      payload: { organizationId: 'org-1', label: 'gone', limitMicros: 1000 },
    });
    const id = (created.json() as { id: string }).id;
    const res = await app.inject({ method: 'DELETE', url: `/api/admin/budgets/${id}` });
    expect(res.statusCode).toBe(204);
  });

  it('GET /api/admin/cost-forecast returns snapshot: null when no data', async () => {
    const res = await app.inject({ method: 'GET', url: '/api/admin/cost-forecast?organizationId=org-1' });
    expect(res.statusCode).toBe(200);
    expect(res.json()).toEqual({ snapshot: null, alerts: [] });
  });
});