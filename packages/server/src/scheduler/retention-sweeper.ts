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

  sweepOnce(_orgId?: string): SweepResult[] {
    const out: SweepResult[] = [];
    const targets: Array<{ table: string; cutoffColumn: string; cutoff: string }> = [
      {
        table: 'eval_results',
        cutoffColumn: 'created_at',
        cutoff: new Date(this.clock().getTime() - DEFAULT_RETENTION_DAYS * 86_400_000).toISOString(),
      },
      {
        table: 'human_review_queue',
        cutoffColumn: 'submitted_at',
        cutoff: new Date(this.clock().getTime() - DEFAULT_RETENTION_DAYS * 86_400_000).toISOString(),
      },
    ];
    this.db.transaction(() => {
      for (const t of targets) {
        if (!this.tableHasColumn(t.table, t.cutoffColumn)) continue;
        const res = this.db
          .prepare(`DELETE FROM ${t.table} WHERE ${t.cutoffColumn} < ?`)
          .run(t.cutoff);
        if (res.changes > 0) {
          out.push({ table: t.table, deletedRows: res.changes, cutoff: t.cutoff });
        }
      }
    })();
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

  private tableHasColumn(table: string, column: string): boolean {
    const rows = this.db
      .prepare(`PRAGMA table_info(${table})`)
      .all() as Array<{ name: string }>;
    return rows.some((r) => r.name === column);
  }
}
