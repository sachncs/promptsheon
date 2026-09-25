import type { EvalRun, EvalResult } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { camelize } from './base.js';

function toRun(row: Record<string, unknown>): EvalRun {
  return camelize(row) as unknown as EvalRun;
}

function toResult(row: Record<string, unknown>): EvalResult {
  const value = camelize(row);
  return { ...value, passed: Boolean(value['passed']) } as unknown as EvalResult;
}

export class EvalRepo {
  constructor(private db: Database.Database) {}

  findRunById(id: string): EvalRun | null {
    const row = this.db.prepare('SELECT * FROM eval_runs WHERE id = ?').get(id) as Record<string, unknown> | undefined;
    return row ? toRun(row) : null;
  }

  findRunByIdInOrg(id: string, organizationId: string): EvalRun | null {
    const row = this.db.prepare(
      `SELECT er.* FROM eval_runs er
       JOIN releases r ON r.id = er.release_id
       JOIN capabilities c ON c.id = r.capability_id
       JOIN projects p ON p.id = c.project_id
       JOIN workspaces w ON w.id = p.workspace_id
       WHERE er.id = ? AND w.org_id = ?`,
    ).get(id, organizationId) as Record<string, unknown> | undefined;
    return row ? toRun(row) : null;
  }

  findRunsByReleaseId(releaseId: string): EvalRun[] {
    return this.db.prepare('SELECT * FROM eval_runs WHERE release_id = ? ORDER BY started_at DESC')
      .all(releaseId).map((row) => toRun(row as Record<string, unknown>));
  }

  findRunsByReleaseIdInOrg(releaseId: string, organizationId: string): EvalRun[] {
    return this.db.prepare(
      `SELECT er.* FROM eval_runs er
       JOIN releases r ON r.id = er.release_id
       JOIN capabilities c ON c.id = r.capability_id
       JOIN projects p ON p.id = c.project_id
       JOIN workspaces w ON w.id = p.workspace_id
       WHERE er.release_id = ? AND w.org_id = ?
       ORDER BY er.started_at DESC`,
    ).all(releaseId, organizationId).map((row) => toRun(row as Record<string, unknown>));
  }

  findMany(opts: { page: number; pageSize: number }): { items: EvalRun[]; total: number } {
    const total = (this.db.prepare('SELECT COUNT(*) as count FROM eval_runs').get() as { count: number }).count;
    const items = this.db.prepare('SELECT * FROM eval_runs ORDER BY started_at DESC LIMIT ? OFFSET ?')
      .all(opts.pageSize, (opts.page - 1) * opts.pageSize).map((row) => toRun(row as Record<string, unknown>));
    return { items, total };
  }

  findManyInOrg(organizationId: string, opts: { page: number; pageSize: number }): { items: EvalRun[]; total: number } {
    const scope = `FROM eval_runs er
      JOIN releases r ON r.id = er.release_id
      JOIN capabilities c ON c.id = r.capability_id
      JOIN projects p ON p.id = c.project_id
      JOIN workspaces w ON w.id = p.workspace_id
      WHERE w.org_id = ?`;
    const total = (this.db.prepare(`SELECT COUNT(*) AS count ${scope}`).get(organizationId) as { count: number }).count;
    const items = this.db.prepare(`SELECT er.* ${scope} ORDER BY er.started_at DESC LIMIT ? OFFSET ?`)
      .all(organizationId, opts.pageSize, (opts.page - 1) * opts.pageSize)
      .map((row) => toRun(row as Record<string, unknown>));
    return { items, total };
  }

  findResultsByRunId(runId: string): EvalResult[] {
    return this.db.prepare('SELECT * FROM eval_results WHERE run_id = ? ORDER BY seq')
      .all(runId).map((row) => toResult(row as Record<string, unknown>));
  }

  createRun(data: { releaseId: string; datasetId: string; scorer: string }, organizationId?: string): EvalRun | null {
    if (organizationId) {
      const valid = this.db.prepare(
        `SELECT 1 FROM releases r
         JOIN capabilities rc ON rc.id = r.capability_id
         JOIN projects rp ON rp.id = rc.project_id
         JOIN workspaces rw ON rw.id = rp.workspace_id
         JOIN datasets d ON d.id = ?
         JOIN capabilities dc ON dc.id = d.capability_id
         JOIN projects dp ON dp.id = dc.project_id
         JOIN workspaces dw ON dw.id = dp.workspace_id
         WHERE r.id = ? AND rw.org_id = ? AND dw.org_id = ?`,
      ).get(data.datasetId, data.releaseId, organizationId, organizationId);
      if (!valid) return null;
    }
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare(`INSERT INTO eval_runs (id, release_id, dataset_id, scorer, score, passed, failed, total, status, started_at, finished_at) VALUES (?, ?, ?, ?, 0, 0, 0, 0, 'running', ?, NULL)`)
      .run(id, data.releaseId, data.datasetId, data.scorer, now);
    return {
      id, releaseId: data.releaseId, datasetId: data.datasetId, scorer: data.scorer,
      score: 0, passed: 0, failed: 0, total: 0, status: 'running',
      startedAt: now, finishedAt: null,
    };
  }

  updateRun(id: string, data: Partial<Pick<EvalRun, 'score' | 'passed' | 'failed' | 'total' | 'status'>>): EvalRun | null {
    const existing = this.findRunById(id);
    if (!existing) return null;
    const merged = { ...existing, ...data, finishedAt: new Date().toISOString() };
    this.db.prepare(`UPDATE eval_runs SET score = ?, passed = ?, failed = ?, total = ?, status = ?, finished_at = ? WHERE id = ?`)
      .run(merged.score, merged.passed, merged.failed, merged.total, merged.status, merged.finishedAt, id);
    return merged;
  }

  addResult(data: { runId: string; caseId: string | null; seq: number; passed: boolean; actual: string; error: string; latencyMs: number }): EvalResult {
    const id = crypto.randomUUID();
    this.db.prepare(`INSERT INTO eval_results (id, run_id, case_id, seq, passed, actual, error, latency_ms) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
      .run(id, data.runId, data.caseId, data.seq, data.passed ? 1 : 0, data.actual, data.error, data.latencyMs);
    return {
      id, runId: data.runId, caseId: data.caseId, seq: data.seq,
      passed: data.passed, actual: data.actual, error: data.error, latencyMs: data.latencyMs,
    };
  }
}
