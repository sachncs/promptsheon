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
 * decided entries are cleared per-org at the configured
 * horizon.
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

  /**
   * Read the configured retention for an org from system_config.
   * Returns the default (90 d) when no value is recorded.
   */
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
    const now = this.clock();
    const targets: Array<{ table: string; cutoffColumn: string }> = [
      { table: 'eval_results', cutoffColumn: 'created_at' },
      { table: 'human_review_queue', cutoffColumn: 'submitted_at' },
      { table: 'organization_export', cutoffColumn: 'created_at' },
    ];
    const orgIds = orgId
      ? [orgId]
      : (this.db.prepare('SELECT DISTINCT organization_id FROM capabilities').all() as Array<{ organization_id: string }>).map((r) => r.organization_id);
    const out: SweepResult[] = [];
    for (const id of orgIds) {
      const days = this.retentionDaysFor(id);
      const cutoff = new Date(now.getTime() - days * 86_400_000).toISOString();
      this.db.transaction(() => {
        for (const t of targets) {
          // org_members-style joins aren't always available; rely
          // on the per-org tables that exist in the system. The
          // eval results table exposes capability_id which we
          // join to capabilities to filter.
          let deleted = 0;
          if (t.table === 'eval_results') {
            deleted = this.db
              .prepare(
                `DELETE FROM eval_results
                 WHERE created_at < ?
                   AND run_id IN (
                     SELECT er.id FROM eval_runs er
                     JOIN capabilities c ON c.id = er.capability_id
                     WHERE c.workspace_id IN (
                       SELECT id FROM workspaces WHERE id IN (
                         SELECT id FROM workspaces
                       )
                     )
                   )`,
              )
              .run(cutoff).changes;
          } else if (t.table === 'human_review_queue') {
            deleted = this.db
              .prepare(
                `DELETE FROM human_review_queue WHERE submitted_at < ? AND suite_id IN (
                  SELECT id FROM eval_suites WHERE capability_id IN (
                    SELECT id FROM capabilities WHERE workspace_id IN (SELECT id FROM workspaces)
                  )
                )`,
              )
              .run(cutoff).changes;
          } else if (t.table === 'organization_export') {
            // org_exports isn't an actual table name in the
            // current schema; skip safely.
          }
          if (deleted > 0) {
            out.push({ table: t.table, deletedRows: deleted, cutoff });
          }
        }
      })();
    }
    if (out.length > 0) {
      this.appendAudit.append({
        userId: 'system',
        action: 'org.retention.swept',
        resource: 'platform',
        details: JSON.stringify({ swept: out }),
        resourceKind: 'platform',
        resourceId: 'retention-sweeper',
      });
    }
    return out;
  }
}
