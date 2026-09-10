import { describe, it, expect, beforeEach } from 'vitest';
import Database from 'better-sqlite3';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { applyMigrations } from '@promptsheon/shared';
import { TraceRepo } from '../src/repos/trace.js';
import { UserAnalyticsRepo } from '../src/repos/user-analytics.js';

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

describe('UserAnalyticsRepo', () => {
  let db: Database.Database;
  let traceRepo: TraceRepo;
  let repo: UserAnalyticsRepo;

  beforeEach(() => {
    db = openDb();
    traceRepo = new TraceRepo(db);
    repo = new UserAnalyticsRepo(db);
  });

  function makeRun(actorId: string | null, tokens: number, cost: number, daysAgo: number) {
    const start = new Date(Date.now() - daysAgo * 86_400_000).toISOString();
    const run = traceRepo.startRun({
      organizationId: 'org-1',
      actorId,
      name: `r-${actorId ?? 'unscoped'}-${daysAgo}`,
    });
    const span = traceRepo.addSpan({
      traceRunId: run.id,
      name: 's',
      kind: 'agent',
      totalTokens: tokens,
      costUsd: cost,
      startTime: start,
    });
    traceRepo.finishSpan(span.id, { endTime: start });
    traceRepo.finalize(run.id, 'success', { tokens, costUsd: cost });
    // Force start_time to the requested day so perDay() picks it up.
    db.prepare('UPDATE trace_runs SET start_time = ? WHERE id = ?').run(start, run.id);
  }

  it('perDay() returns one row per day with tokens + cost', () => {
    makeRun('alice', 100, 0.001, 0);
    makeRun('alice', 50, 0.0005, 1);
    const rows = repo.perDay('alice', 7);
    expect(rows.length).toBe(2);
    expect(rows.reduce((acc, r) => acc + r.tokens, 0)).toBe(150);
  });

  it('leaderboardByOrg ranks users by tokens and excludes unscoped', () => {
    makeRun('alice', 1000, 0.01, 0);
    makeRun('alice', 1000, 0.01, 0);
    makeRun('bob', 100, 0.001, 0);
    makeRun(null, 9_999, 0.99, 0); // unscoped, excluded
    const rows = repo.leaderboardByOrg('org-1', { days: 1 });
    expect(rows[0]?.actorId).toBe('alice');
    expect(rows[0]?.tokens).toBe(2_000);
    expect(rows[1]?.actorId).toBe('bob');
    expect(rows.length).toBe(2);
  });

  it('orgTotals sums tokens + cost + runs + active days', () => {
    makeRun('alice', 100, 0.001, 0);
    makeRun('alice', 200, 0.002, 0);
    makeRun('bob', 50, 0.0005, 0);
    const totals = repo.orgTotals('org-1', 7);
    expect(totals.runs).toBe(3);
    expect(totals.tokens).toBe(350);
    expect(totals.cost).toBeCloseTo(0.0035, 6);
    expect(totals.activeDays).toBe(1);
  });
});
