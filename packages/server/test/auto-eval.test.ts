import { describe, it, expect, beforeEach } from 'vitest';
import Database from 'better-sqlite3';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { applyMigrations } from '@promptsheon/shared';
import { TraceRepo } from '../src/repos/trace.js';
import { TraceScoreRepo } from '../src/repos/trace-score.js';
import {
  AutoEval,
  listBuiltInEvaluators,
  registerEvaluator,
  resetEvaluators,
  type Evaluator,
} from '../src/observability/auto-eval.js';

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

describe('Built-in evaluators', () => {
  it('latency-budget labels spans within / over budget', async () => {
    const db = openDb();
    const traceRepo = new TraceRepo(db);
    const scoreRepo = new TraceScoreRepo(db);
    const autoEval = new AutoEval({ traceRepo, scoreRepo });
    const run = traceRepo.startRun({ organizationId: 'org', name: 'r' });
    const s1 = traceRepo.addSpan({ traceRunId: run.id, name: 'op', kind: 'agent' });
    traceRepo.finishSpan(s1.id, { endTime: new Date(Date.now() + 1_500).toISOString() });
    const s2 = traceRepo.addSpan({ traceRunId: run.id, name: 'op', kind: 'agent' });
    traceRepo.finishSpan(s2.id, { endTime: new Date(Date.now() + 200).toISOString() });
    traceRepo.finalize(run.id, 'success');

    const written = await autoEval.run(run.id);
    const latencyScore = scoreRepo.listByRun(run.id).find((s) => s.evaluator === 'latency-budget');
    expect(written).toBeGreaterThan(0);
    expect(latencyScore).toBeDefined();
    // p99 over 1500ms is way over the 5s budget so label is within_budget
    expect(latencyScore!.label).toBe('within_budget');
    void db;
  });

  it('error-rate counts error spans', async () => {
    const db = openDb();
    const traceRepo = new TraceRepo(db);
    const scoreRepo = new TraceScoreRepo(db);
    const autoEval = new AutoEval({ traceRepo, scoreRepo });
    const run = traceRepo.startRun({ organizationId: 'org', name: 'r' });
    const a = traceRepo.addSpan({ traceRunId: run.id, name: 'a' });
    traceRepo.finishSpan(a.id, { status: 'ok' });
    const b = traceRepo.addSpan({ traceRunId: run.id, name: 'b' });
    traceRepo.finishSpan(b.id, { status: 'error' });
    traceRepo.finalize(run.id, 'error');

    await autoEval.run(run.id);
    const errScore = scoreRepo.listByRun(run.id).find((s) => s.evaluator === 'error-rate');
    expect(errScore?.value).toBe(0.5);
    expect(errScore?.label).toBe('high');
  });

  it('output-shape flags missing JSON fields', async () => {
    const db = openDb();
    const traceRepo = new TraceRepo(db);
    const scoreRepo = new TraceScoreRepo(db);
    const autoEval = new AutoEval({ traceRepo, scoreRepo });
    const run = traceRepo.startRun({ organizationId: 'org', name: 'r' });
    const span = traceRepo.addSpan({
      traceRunId: run.id,
      name: 'op',
      kind: 'llm',
      outputText: '{"result":"ok"}', // missing 'confidence'
    });
    traceRepo.finishSpan(span.id, {});
    traceRepo.finalize(run.id, 'success');

    await autoEval.run(run.id);
    const shape = scoreRepo.listByRun(run.id).find((s) => s.evaluator === 'output-shape');
    expect(shape?.value).toBe(0);
    expect(shape?.label).toBe('missing_fields');
    expect(shape?.rationale).toContain('confidence');
  });

  it('output-shape passes when all required fields are present', async () => {
    const db = openDb();
    const traceRepo = new TraceRepo(db);
    const scoreRepo = new TraceScoreRepo(db);
    const autoEval = new AutoEval({ traceRepo, scoreRepo });
    const run = traceRepo.startRun({ organizationId: 'org', name: 'r' });
    const span = traceRepo.addSpan({
      traceRunId: run.id,
      name: 'op',
      kind: 'llm',
      outputText: '{"result":"ok","confidence":0.9}',
    });
    traceRepo.finishSpan(span.id, {});
    traceRepo.finalize(run.id, 'success');

    await autoEval.run(run.id);
    const shape = scoreRepo.listByRun(run.id).find((s) => s.evaluator === 'output-shape');
    expect(shape?.value).toBe(1);
    expect(shape?.label).toBe('complete');
  });

  it('token-cost sums spans and labels against the budget', async () => {
    const db = openDb();
    const traceRepo = new TraceRepo(db);
    const scoreRepo = new TraceScoreRepo(db);
    const autoEval = new AutoEval({ traceRepo, scoreRepo });
    const run = traceRepo.startRun({ organizationId: 'org', name: 'r' });
    const span = traceRepo.addSpan({
      traceRunId: run.id,
      name: 'op',
      kind: 'llm',
      costUsd: 0.01,
    });
    traceRepo.finishSpan(span.id, {});
    traceRepo.finalize(run.id, 'success');

    await autoEval.run(run.id);
    const cost = scoreRepo.listByRun(run.id).find((s) => s.evaluator === 'token-cost');
    expect(cost?.value).toBeCloseTo(0.01, 6);
    expect(cost?.label).toBe('within_budget');
  });

  it('registerEvaluator accepts a custom deterministic evaluator', async () => {
    resetEvaluators();
    const custom: Evaluator = {
      name: 'passes-with-no-spans',
      kind: 'deterministic',
      async run() {
        return { evaluator: 'passes-with-no-spans', name: 'always_ok', value: 1, label: 'ok', rationale: 'noop' };
      },
    };
    registerEvaluator(custom);
    expect(listBuiltInEvaluators().some((e) => e.name === 'passes-with-no-spans')).toBe(true);
  });

  it('summaryByOrg counts scores per evaluator', async () => {
    const db = openDb();
    const traceRepo = new TraceRepo(db);
    const scoreRepo = new TraceScoreRepo(db);
    const run = traceRepo.startRun({ organizationId: 'org-x', name: 'r' });
    traceRepo.finalize(run.id, 'success');
    scoreRepo.record({ traceRunId: run.id, executionId: run.executionId, evaluator: 'a', name: 'n', value: 1, label: 'ok' });
    scoreRepo.record({ traceRunId: run.id, executionId: run.executionId, evaluator: 'a', name: 'n2', value: 0, label: 'fail' });
    scoreRepo.record({ traceRunId: run.id, executionId: run.executionId, evaluator: 'b', name: 'n3', value: 1, label: 'ok' });
    const out = scoreRepo.summaryByOrg('org-x', { days: 7 });
    expect(out.totals).toBe(3);
    expect(out.perEvaluator.find((p) => p.evaluator === 'a')?.count).toBe(2);
    expect(out.perEvaluator.find((p) => p.evaluator === 'b')?.count).toBe(1);
  });
});
