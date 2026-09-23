import type Database from 'better-sqlite3';

const DEFAULT_RETENTION_DAYS = 90;

interface SweepResult {
  table: string;
  deletedRows: number;
  cutoff: string;
}

interface AuditAppender {
  append(entry: {
    userId: string;
    action: string;
    resource: string;
    details: string;
    resourceKind: string;
    resourceId: string;
  }): void;
}

/**
 * RetentionSweeper — platform-level cron that prunes rows
 * older than the configured retention window.
 *
 * The audit chain itself is hash-linked and append-only, so
 * audit_entries are NOT pruned. Eval results and human-review
 * decided entries are cleared per workspace at the configured
 * horizon; the workspace boundary is the closest scoped
 * identifier the eval tables carry.
 *
 * Per-org retention overrides live in the system_config table
 * (`org.retention.days.<orgId>`). Falling back to the
 * DEFAULT_RETENTION_DAYS keeps the sweep safe for orgs that
 * haven't pinned a value.
 */
export class RetentionSweeper {
  private interval: ReturnType<typeof setInterval> | null = null;

  constructor(
    private db: Database.Database,
    private appendAudit: AuditAppender,
    private clock: () => Date = () => new Date(),
  ) {}

  start(periodMs = 6 * 60 * 60 * 1000): void {
    void this.sweepOnce();
    this.interval = setInterval(() => {
      void this.sweepOnce();
    }, periodMs);
  }

  stop(): void {
    if (this.interval) clearInterval(this.interval);
    this.interval = null;
  }

  retentionDaysFor(orgId: string): number {
    const row = this.db
      .prepare("SELECT value FROM system_config WHERE key = ?")
      .get(`org.retention.days.${orgId}`) as { value: string } | undefined;
    if (!row) return DEFAULT_RETENTION_DAYS;
    const n = Number.parseInt(row.value, 10);
    return Number.isFinite(n) && n > 0 ? n : DEFAULT_RETENTION_DAYS;
  }

  setRetentionDaysFor(orgId: string, days: number): void {
    this.db
      .prepare(
        `INSERT INTO system_config (key, value, updated_by) VALUES (?, ?, 'system')
         ON CONFLICT (key) DO UPDATE SET value = excluded.value, write_ts = write_ts + 1`,
      )
      .run(`org.retention.days.${orgId}`, String(days));
  }

  sweepOnce(orgId?: string): SweepResult[] {
    const out: SweepResult[] = [];
    const organizations = orgId ? [orgId] : this.organizationIds();
    this.db.transaction(() => {
      for (const currentOrgId of organizations) {
        const cutoff = new Date(this.clock().getTime() - this.retentionDaysFor(currentOrgId) * 86_400_000).toISOString();
        const evalResults = this.db
          .prepare(
            `DELETE FROM eval_results
             WHERE EXISTS (
                 SELECT 1
                 FROM eval_runs er
                 JOIN releases r ON r.id = er.release_id
                 JOIN capabilities c ON c.id = r.capability_id
                 JOIN projects p ON p.id = c.project_id
                 JOIN workspaces w ON w.id = p.workspace_id
                 WHERE er.id = eval_results.run_id AND er.started_at < ? AND w.org_id = ?
               )`,
          )
          .run(cutoff, currentOrgId);
        if (evalResults.changes > 0) {
          out.push({ table: 'eval_results', deletedRows: evalResults.changes, cutoff });
        }

        const reviews = this.db
          .prepare(
            `DELETE FROM human_review_queue
             WHERE submitted_at < ?
               AND EXISTS (
                 SELECT 1
                 FROM eval_suites es
                 JOIN capabilities c ON c.id = es.capability_id
                 JOIN projects p ON p.id = c.project_id
                 JOIN workspaces w ON w.id = p.workspace_id
                 WHERE es.id = human_review_queue.suite_id AND w.org_id = ?
               )`,
          )
          .run(cutoff, currentOrgId);
        if (reviews.changes > 0) {
          out.push({ table: 'human_review_queue', deletedRows: reviews.changes, cutoff });
        }
      }
    })();
    if (out.length > 0) {
      this.appendAudit.append({
        userId: 'system',
        action: 'org.retention.swept',
        resource: 'platform',
        details: JSON.stringify({ organizationId: orgId ?? null, swept: out }),
        resourceKind: 'platform',
        resourceId: 'retention-sweeper',
      });
    }
    return out;
  }

  private organizationIds(): string[] {
    const rows = this.db
      .prepare('SELECT DISTINCT org_id FROM workspaces WHERE org_id IS NOT NULL')
      .all() as Array<{ org_id: string }>;
    return rows.map((row) => row.org_id);
  }
}
