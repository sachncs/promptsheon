import { describe, it, expect, beforeEach } from 'vitest';
import Database from 'better-sqlite3';
import { applyMigrations } from '@promptsheon/shared';
import {
  buildDailySeries,
  linearRegression,
  periodBounds,
} from '../src/analysis/forecast.js';
import { CostForecastService } from '../src/analysis/forecast.js';
import { CostBudgetRepo } from '../src/repos/budget.js';
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

function makeDb(): Database.Database {
  const db = new Database(':memory:');
  db.pragma('foreign_keys = ON');
  applyMigrations(db, loadAllMigrations());
  // Bootstrap minimal org + workspace + project + capability chain
  // so the rollupsForOrg join can resolve.
  db.prepare(
    `INSERT INTO orgs (id, name, slug, residency, created_at, updated_at) VALUES ('org-1', 'Test', 'test', 'local', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
  ).run();
  db.prepare(
    `INSERT INTO workspaces (id, name, organization, org_id, created_at, updated_at) VALUES ('ws1', 'WS', '', 'org-1', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
  ).run();
  db.prepare(
    `INSERT INTO projects (id, workspace_id, name, description, created_at, updated_at) VALUES ('proj1', 'ws1', 'P', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
  ).run();
  db.prepare(
    `INSERT INTO capabilities (id, project_id, name, description, created_at, updated_at) VALUES ('cap1', 'proj1', 'C', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
  ).run();
  return db;
}

describe('linearRegression', () => {
  it('returns null for fewer than two points', () => {
    expect(linearRegression([])).toBeNull();
    expect(linearRegression([{ x: 0, y: 1 }])).toBeNull();
  });
  it('returns null when x has no variance', () => {
    expect(linearRegression([{ x: 1, y: 1 }, { x: 1, y: 2 }])).toBeNull();
  });
  it('fits a known linear function exactly (r²=1, residualStdDev=0)', () => {
    // y = 2x + 3
    const points = [
      { x: 0, y: 3 },
      { x: 1, y: 5 },
      { x: 2, y: 7 },
      { x: 3, y: 9 },
    ];
    const fit = linearRegression(points)!;
    expect(fit.intercept).toBeCloseTo(3, 6);
    expect(fit.slope).toBeCloseTo(2, 6);
    expect(fit.r2).toBeCloseTo(1, 6);
    expect(fit.residualStdDev).toBeLessThan(1e-9);
  });
  it('reports a low r² for noisy data', () => {
    const points = [
      { x: 0, y: 1 },
      { x: 1, y: 10 },
      { x: 2, y: 2 },
      { x: 3, y: 9 },
    ];
    const fit = linearRegression(points)!;
    expect(fit.r2).toBeLessThan(0.5);
  });
});

describe('buildDailySeries', () => {
  it('returns N entries when windowDays=N', () => {
    const series = buildDailySeries([], 30);
    expect(series).toHaveLength(30);
  });
  it('fills missing days with zero spend', () => {
    const series = buildDailySeries([], 7);
    expect(series.every((p) => p.y === 0)).toBe(true);
  });
  it('uses provided rollups on the matching day', () => {
    const todayIso = new Date().toISOString().slice(0, 10);
    const series = buildDailySeries([{ day: todayIso, costMicros: 1234 }], 3);
    const todayEntry = series.find((p) => p.day === todayIso);
    expect(todayEntry?.y).toBe(1234);
  });
});

describe('periodBounds', () => {
  it('monthly bounds are anchored to UTC midnight', () => {
    const b = periodBounds('monthly', new Date(Date.UTC(2026, 6, 15, 23, 0)));
    expect(b.start).toBe('2026-07-01');
    expect(b.end).toBe('2026-07-31');
    expect(b.days).toBe(31);
    expect(b.todayIndex).toBe(14);
  });
  it('weekly bounds start on Monday', () => {
    // 2026-08-26 is a Wednesday; Monday is 2026-08-24
    const b = periodBounds('weekly', new Date(Date.UTC(2026, 7, 26, 12, 0)));
    expect(b.start).toBe('2026-08-24');
    expect(b.end).toBe('2026-08-30');
    expect(b.days).toBe(7);
  });
});

describe('CostForecastService', () => {
  let db: Database.Database;
  let budgetRepo: CostBudgetRepo;
  let service: CostForecastService;

  beforeEach(() => {
    db = makeDb();
    budgetRepo = new CostBudgetRepo(db);
    service = new CostForecastService(db, {
      listBudgets: (orgId) => budgetRepo.listForOrg(orgId),
      updateLastAlerted: (id, ts) => budgetRepo.updateLastAlerted(id, ts),
      persistSnapshot: (snap) => budgetRepo.insertForecastSnapshot(snap),
    });
  });

  it('returns null when the org has no rollups', () => {
    expect(service.compute('org-1')).toBeNull();
  });

  it('computes a snapshot from seeded rollups', () => {
    const today = new Date();
    for (let i = 0; i < 14; i += 1) {
      const d = new Date(today);
      d.setUTCDate(d.getUTCDate() - i);
      const day = d.toISOString().slice(0, 10);
      db.prepare(
        `INSERT INTO capability_cost_rollups (capability_id, day, input_tokens, output_tokens, cost_micros, executions)
         VALUES ('cap1', ?, ?, ?, ?, ?)`,
      ).run(day, 1000 * i, 500 * i, 100_000 * i, 10 * i);
    }
    const snapshot = service.snapshot('org-1', { windowDays: 14 });
    expect(snapshot).not.toBeNull();
    expect(snapshot!.windowDays).toBe(14);
    expect(snapshot!.projectedMicros).toBeGreaterThan(0);
    expect(snapshot!.bandLowMicros).toBeLessThanOrEqual(snapshot!.projectedMicros);
    expect(snapshot!.bandHighMicros).toBeGreaterThanOrEqual(snapshot!.projectedMicros);
    // The snapshot was persisted for the dashboard.
    const latest = budgetRepo.latestForecast('org-1');
    expect(latest).not.toBeNull();
  });

  it('fires an alert when projected spend exceeds the budget threshold', () => {
    const budget = budgetRepo.create({
      organizationId: 'org-1',
      label: 'monthly-cap',
      period: 'monthly',
      limitMicros: 100_000, // $0.10
      alertThreshold: 0.5,
    });
    const today = new Date();
    for (let i = 0; i < 14; i += 1) {
      const d = new Date(today);
      d.setUTCDate(d.getUTCDate() - i);
      const day = d.toISOString().slice(0, 10);
      db.prepare(
        `INSERT INTO capability_cost_rollups (capability_id, day, input_tokens, output_tokens, cost_micros, executions)
         VALUES ('cap1', ?, ?, ?, ?, ?)`,
      ).run(day, 1000, 500, 1_000_000, 5); // 1M micros = $1/day for 14 days
    }
    const result = service.compute('org-1');
    expect(result).not.toBeNull();
    expect(result!.alerts.length).toBeGreaterThan(0);
    const alert = result!.alerts[0]!;
    expect(alert.budgetId).toBe(budget.id);
    expect(alert.fraction).toBeGreaterThanOrEqual(0.5);
  });

  it('does not fire when the projection is below the threshold', () => {
    budgetRepo.create({
      organizationId: 'org-1',
      label: 'loose',
      period: 'monthly',
      limitMicros: 100_000_000, // $100
      alertThreshold: 0.95,
    });
    const today = new Date();
    for (let i = 0; i < 14; i += 1) {
      const d = new Date(today);
      d.setUTCDate(d.getUTCDate() - i);
      const day = d.toISOString().slice(0, 10);
      db.prepare(
        `INSERT INTO capability_cost_rollups (capability_id, day, input_tokens, output_tokens, cost_micros, executions)
         VALUES ('cap1', ?, ?, ?, ?, ?)`,
      ).run(day, 100, 50, 100_000, 5);
    }
    const result = service.compute('org-1');
    expect(result).not.toBeNull();
    expect(result!.alerts).toHaveLength(0);
  });

  it('skips disabled budgets', () => {
    const b = budgetRepo.create({
      organizationId: 'org-1',
      label: 'disabled',
      period: 'monthly',
      limitMicros: 100_000_000_000,
      alertThreshold: 0.01,
      enabled: false,
    });
    const today = new Date();
    for (let i = 0; i < 14; i += 1) {
      const d = new Date(today);
      d.setUTCDate(d.getUTCDate() - i);
      const day = d.toISOString().slice(0, 10);
      db.prepare(
        `INSERT INTO capability_cost_rollups (capability_id, day, input_tokens, output_tokens, cost_micros, executions)
         VALUES ('cap1', ?, ?, ?, ?, ?)`,
      ).run(day, 100, 50, 1_000_000, 5);
    }
    const result = service.compute('org-1');
    expect(result!.alerts.find((a) => a.budgetId === b.id)).toBeUndefined();
  });
});

describe('CostBudgetRepo', () => {
  let db: Database.Database;
  let repo: CostBudgetRepo;

  beforeEach(() => {
    db = makeDb();
    repo = new CostBudgetRepo(db);
  });

  it('creates and lists a budget', () => {
    const created = repo.create({
      organizationId: 'org-1',
      label: 'monthly',
      period: 'monthly',
      limitMicros: 5_000_000,
      alertThreshold: 0.8,
    });
    expect(created.label).toBe('monthly');
    expect(repo.listForOrg('org-1')).toHaveLength(1);
  });

  it('updates fields and bumps updated_at', async () => {
    const b = repo.create({
      organizationId: 'org-1',
      label: 'a',
      limitMicros: 1_000,
    });
    await new Promise((r) => setTimeout(r, 5));
    const updated = repo.update(b.id, { limitMicros: 2_000 });
    expect(updated?.limitMicros).toBe(2_000);
    expect(updated!.updatedAt > b.updatedAt).toBe(true);
  });

  it('deletes a budget', () => {
    const b = repo.create({ organizationId: 'org-1', label: 'a', limitMicros: 1 });
    expect(repo.delete(b.id)).toBe(true);
    expect(repo.findById(b.id)).toBeNull();
  });

  it('updateLastAlerted is a no-op- on unknown id', () => {
    expect(() =>
      repo.updateLastAlerted('does-not-exist', new Date().toISOString()),
    ).not.toThrow();
  });
});