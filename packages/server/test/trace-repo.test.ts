import { describe, it, expect, beforeEach } from 'vitest';
import Database from 'better-sqlite3';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { applyMigrations } from '@promptsheon/shared';
import { TraceRepo } from '../src/repos/trace.js';

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

function openDb(): Database.Database {
  const db = new Database(':memory:');
  db.pragma('foreign_keys = ON');
  applyMigrations(db, loadAllMigrations());
  return db;
}

describe('TraceRepo', () => {
  let db: Database.Database;
  let repo: TraceRepo;

  beforeEach(() => {
    db = openDb();
    repo = new TraceRepo(db);
  });

  it('startRun + findById round-trips', () => {
    const run = repo.startRun({
      organizationId: '00000000-0000-4000-8000-000000000001',
      name: 'test trace',
      model: 'gpt-4',
      attributes: { foo: 'bar' },
    });
    expect(run.organizationId).toBe('00000000-0000-4000-8000-000000000001');
    expect(run.status).toBe('running');
    const fetched = repo.findById(run.id);
    expect(fetched?.name).toBe('test trace');
    expect(fetched?.attributes['foo']).toBe('bar');
  });

  it('addSpan + findSpansByRun returns children in time order', () => {
    const run = repo.startRun({ organizationId: 'org-1', name: 'rt' });
    const s1 = repo.addSpan({ traceRunId: run.id, name: 'first', kind: 'agent' });
    const s2 = repo.addSpan({ traceRunId: run.id, name: 'second', kind: 'llm' });
    expect(s1.name).toBe('first');
    expect(s2.name).toBe('second');
    const spans = repo.findSpansByRun(run.id);
    expect(spans.length).toBe(2);
    expect(spans[0]?.id).toBe(s1.id);
    expect(spans[1]?.id).toBe(s2.id);
    expect(spans[1]?.kind).toBe('llm');
  });

  it('finishSpan records status + duration', () => {
    const run = repo.startRun({ organizationId: 'org-1', name: 'dur' });
    const span = repo.addSpan({ traceRunId: run.id, name: 'op' });
    repo.finishSpan(span.id, { endTime: '2026-01-01T00:00:00.000Z', status: 'ok' });
    const spans = repo.findSpansByRun(run.id);
    expect(spans[0]?.status).toBe('ok');
    expect(spans[0]?.endTime).toBe('2026-01-01T00:00:00.000Z');
  });

  it('finalize aggregates token + cost from child spans', () => {
    const run = repo.startRun({ organizationId: 'org-1', name: 'totals' });
    repo.addSpan({ traceRunId: run.id, name: 'a', totalTokens: 100, costUsd: 0.001 });
    repo.addSpan({ traceRunId: run.id, name: 'b', totalTokens: 250, costUsd: 0.005 });
    repo.finalize(run.id, 'success');
    const fetched = repo.findById(run.id);
    expect(fetched?.status).toBe('success');
    expect(fetched?.totalTokens).toBe(350);
    expect(fetched?.totalCostUsd).toBeCloseTo(0.006, 5);
  });

  it('listByOrg supports filters and pagination', () => {
    for (let i = 0; i < 5; i++) {
      const r = repo.startRun({ organizationId: 'org-A', name: `run-${i}`, environment: 'dev' });
      repo.finalize(r.id, 'success');
    }
    for (let i = 0; i < 3; i++) {
      const r = repo.startRun({ organizationId: 'org-B', name: `run-${i}`, environment: 'prod' });
      repo.finalize(r.id, 'success');
    }
    const orgA = repo.listByOrg('org-A', { page: 1, pageSize: 10 });
    expect(orgA.total).toBe(5);
    expect(orgA.items.every((r) => r.organizationId === 'org-A')).toBe(true);
    const envProd = repo.listByOrg('org-A', { environment: 'prod', page: 1, pageSize: 10 });
    expect(envProd.total).toBe(0);
    const onlyProd = repo.listByOrg('org-B', { environment: 'prod', page: 1, pageSize: 10 });
    expect(onlyProd.total).toBe(3);
  });

  it('rollupByOrg aggregates per day', () => {
    const today = new Date().toISOString().slice(0, 10);
    const r1 = repo.startRun({ organizationId: 'org-1', name: 'a', model: 'gpt-4' });
    repo.addSpan({ traceRunId: r1.id, name: 's1', totalTokens: 100, costUsd: 0.002 });
    repo.finalize(r1.id, 'success');
    const r2 = repo.startRun({ organizationId: 'org-1', name: 'b' });
    repo.addSpan({ traceRunId: r2.id, name: 's2', totalTokens: 50, costUsd: 0.001 });
    repo.finalize(r2.id, 'success');
    const rows = repo.rollupByOrg('org-1', { days: 7 });
    expect(rows.length).toBe(1);
    expect(rows[0]?.day).toBe(today);
    expect(rows[0]?.tokens).toBe(150);
    expect(rows[0]?.runs).toBe(2);
  });
});
