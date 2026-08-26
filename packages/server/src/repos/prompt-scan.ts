import type Database from 'better-sqlite3';
import { randomUUID } from 'node:crypto';
import { BaseRepo } from './base.js';

export type PromptVerdict = 'clean' | 'warn' | 'block';
export type FindingSeverity = 'info' | 'warn' | 'block';

export interface Finding {
  rule: string;
  severity: FindingSeverity;
  message: string;
  range?: { start: number; end: number };
  snippet?: string;
}

export interface PromptScan {
  id: string;
  organizationId: string;
  actorId: string | null;
  resourceKind: string;
  resourceId: string;
  verdict: PromptVerdict;
  findingsCount: number;
  findings: Finding[];
  createdAt: string;
}

interface PromptScanRow {
  id: string;
  organization_id: string;
  actor_id: string | null;
  resource_kind: string;
  resource_id: string;
  verdict: string;
  findings_count: number;
  findings: string;
  created_at: string;
}

/**
 * PromptScanRepo — T2-3 persistence. Every save of a manifest
 * (or any other user-authored content) writes one scan row. The
 * audit report generator (T2-4) reads this table to populate the
 * per-quarter evidence pack.
 */
export class PromptScanRepo extends BaseRepo<PromptScan> {
  constructor(db: Database.Database) {
    super(db, 'prompt_scans');
  }

  record(input: {
    organizationId: string;
    actorId?: string | null;
    resourceKind: string;
    resourceId: string;
    verdict: PromptVerdict;
    findings: Finding[];
  }): PromptScan {
    const id = randomUUID();
    const now = new Date().toISOString();
    this.db
      .prepare(
        `INSERT INTO prompt_scans (id, organization_id, actor_id, resource_kind,
          resource_id, verdict, findings_count, findings, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(
        id,
        input.organizationId,
        input.actorId ?? null,
        input.resourceKind,
        input.resourceId,
        input.verdict,
        input.findings.length,
        JSON.stringify(input.findings),
        now,
      );
    const row = this.db
      .prepare('SELECT * FROM prompt_scans WHERE id = ?')
      .get(id) as PromptScanRow;
    return {
      id: row.id,
      organizationId: row.organization_id,
      actorId: row.actor_id,
      resourceKind: row.resource_kind,
      resourceId: row.resource_id,
      verdict: row.verdict as PromptVerdict,
      findingsCount: row.findings_count,
      findings: JSON.parse(row.findings) as Finding[],
      createdAt: row.created_at,
    };
  }

  listByOrg(
    organizationId: string,
    opts: { days?: number; verdict?: PromptVerdict; limit?: number } = {},
  ): PromptScan[] {
    const days = opts.days ?? 30;
    const limit = Math.min(opts.limit ?? 100, 500);
    const since = new Date(Date.now() - days * 86_400_000).toISOString();
    const conds = ['organization_id = ?', 'created_at >= ?'];
    const args: unknown[] = [organizationId, since];
    if (opts.verdict) {
      conds.push('verdict = ?');
      args.push(opts.verdict);
    }
    const rows = this.db
      .prepare(
        `SELECT * FROM prompt_scans WHERE ${conds.join(' AND ')}
         ORDER BY created_at DESC LIMIT ?`,
      )
      .all(...args, limit) as PromptScanRow[];
    return rows.map((r) => ({
      id: r.id,
      organizationId: r.organization_id,
      actorId: r.actor_id,
      resourceKind: r.resource_kind,
      resourceId: r.resource_id,
      verdict: r.verdict as PromptVerdict,
      findingsCount: r.findings_count,
      findings: JSON.parse(r.findings) as Finding[],
      createdAt: r.created_at,
    }));
  }

  listByResource(resourceKind: string, resourceId: string): PromptScan[] {
    const rows = this.db
      .prepare(
        'SELECT * FROM prompt_scans WHERE resource_kind = ? AND resource_id = ? ORDER BY created_at DESC',
      )
      .all(resourceKind, resourceId) as PromptScanRow[];
    return rows.map((r) => ({
      id: r.id,
      organizationId: r.organization_id,
      actorId: r.actor_id,
      resourceKind: r.resource_kind,
      resourceId: r.resource_id,
      verdict: r.verdict as PromptVerdict,
      findingsCount: r.findings_count,
      findings: JSON.parse(r.findings) as Finding[],
      createdAt: r.created_at,
    }));
  }

  summaryByOrg(organizationId: string, days = 30): {
    total: number;
    byVerdict: Record<PromptVerdict, number>;
  } {
    const since = new Date(Date.now() - days * 86_400_000).toISOString();
    const row = this.db
      .prepare(
        `SELECT verdict, COUNT(*) AS c FROM prompt_scans
         WHERE organization_id = ? AND created_at >= ?
         GROUP BY verdict`,
      )
      .all(organizationId, since) as Array<{ verdict: PromptVerdict; c: number }>;
    const byVerdict: Record<PromptVerdict, number> = { clean: 0, warn: 0, block: 0 };
    let total = 0;
    for (const r of row) {
      byVerdict[r.verdict] = r.c;
      total += r.c;
    }
    return { total, byVerdict };
  }
}
