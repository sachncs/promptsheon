import type Database from 'better-sqlite3';
import type { EvalRun, EvalResult } from '@promptsheon/shared';

export class EvalRepo {
  constructor(private db: Database.Database) {}

  findRunById(id: string): EvalRun | null {
    return this.db.prepare('SELECT * FROM eval_runs WHERE id = ?').get(id) as EvalRun | null;
  }

  findRunsByReleaseId(releaseId: string): EvalRun[] {
    return this.db.prepare('SELECT * FROM eval_runs WHERE release_id = ? ORDER BY started_at DESC')
      .all(releaseId) as EvalRun[];
  }

  findResultsByRunId(runId: string): EvalResult[] {
    return this.db.prepare('SELECT * FROM eval_results WHERE run_id = ? ORDER BY seq')
      .all(runId) as EvalResult[];
  }

  createRun(data: { releaseId: string; datasetId: string; scorer: string }): EvalRun {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare('INSERT INTO eval_runs (id, release_id, dataset_id, scorer, status, started_at) VALUES (?, ?, ?, ?, ?, ?)')
      .run(id, data.releaseId, data.datasetId, data.scorer, 'running', now);
    return { id, releaseId: data.releaseId, datasetId: data.datasetId, scorer: data.scorer, score: 0, passed: 0, failed: 0, total: 0, status: 'running', startedAt: now, finishedAt: null };
  }

  updateRun(id: string, data: Partial<Pick<EvalRun, 'score' | 'passed' | 'failed' | 'total' | 'status'>>): EvalRun | null {
    const existing = this.findRunById(id);
    if (!existing) throw new Error(`eval_run ${id} not found`);
    const finishedAt = data.status && data.status !== 'running' ? new Date().toISOString() : existing.finishedAt;
    const fields: string[] = [];
    const values: unknown[] = [];
    for (const [key, value] of Object.entries(data)) {
      if (value === undefined) continue;
      const col = key.replace(/([A-Z])/g, '_$1').toLowerCase();
      fields.push(`${col} = ?`);
      values.push(value);
    }
    if (finishedAt !== existing.finishedAt) {
      fields.push('finished_at = ?');
      values.push(finishedAt);
    }
    if (fields.length > 0) {
      values.push(id);
      this.db.prepare(`UPDATE eval_runs SET ${fields.join(', ')} WHERE id = ?`).run(...values);
    }
    return { ...existing, ...data, finishedAt };
  }

  addResult(data: { runId: string; caseId: string | null; seq: number; passed: boolean; actual: string; error: string; latencyMs: number }): EvalResult {
    const id = crypto.randomUUID();
    this.db.prepare('INSERT INTO eval_results (id, run_id, case_id, seq, passed, actual, error, latency_ms) VALUES (?, ?, ?, ?, ?, ?, ?, ?)')
      .run(id, data.runId, data.caseId, data.seq, data.passed ? 1 : 0, data.actual, data.error, data.latencyMs);
    return { id, runId: data.runId, caseId: data.caseId, seq: data.seq, passed: data.passed, actual: data.actual, error: data.error, latencyMs: data.latencyMs };
  }

  findMany(opts: { page: number; pageSize: number }): { items: EvalRun[]; total: number } {
    const total = (this.db.prepare('SELECT COUNT(*) as count FROM eval_runs').get() as { count: number }).count;
    const items = this.db.prepare('SELECT * FROM eval_runs ORDER BY started_at DESC LIMIT ? OFFSET ?')
      .all(opts.pageSize, (opts.page - 1) * opts.pageSize) as EvalRun[];
    return { items, total };
  }
}
