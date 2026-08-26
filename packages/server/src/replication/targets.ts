/**
 * Audit-chain replication: ship WAL frames from a primary
 * promptsheon SQLite to a replica so the append-only audit log
 * has a hot standby. The rest of the metadata (manifests,
 * capabilities, etc.) stays on the primary; the audit chain is
 * the only thing SOC2 / HIPAA auditors require durable
 * multi-region copies of.
 *
 * Design notes
 * ============
 *
 * better-sqlite3 exposes the underlying SQLite WAL via
 * `sqlite3_db_filename` + `PRAGMA wal_checkpoint(TRUNCATE)` —
 * we read frames out-of-band by holding a separate connection
 * to the same file and walking the WAL header. That is *not*
 * what production Litestream does (it copies raw WAL pages
 * outside SQLite); here we trade correctness-with-effort for
 * simplicity: the primary calls our `onFrame` hook after each
 * append, we ship the frame, the replica applies it in a
 * single transaction.
 *
 * `frame` here is the JSON we already serialize into the audit
 * chain's hashInput. The replicator doesn't try to replay raw
 * SQLite WAL — it ships the application-level "audit entry"
 * that the primary just inserted. The replica reconstructs its
 * own audit_chain_state from the frame stream.
 *
 * Targets
 * =======
 *
 * `FileTarget` appends to a sibling SQLite database (the replica).
 * `HttpTarget` POSTs each frame to a remote URL. Both implement
 * the same `ReplicationTarget` interface so the primary doesn't
 * care which one is wired up.
 */
import type Database from 'better-sqlite3';

export interface AuditFrame {
  /** Monotonic id assigned by the primary's audit_entries.rowid. */
  rowid: number;
  /** SHA-256 hex of the previous frame's `entry_hash`. Empty for the genesis frame. */
  previousHash: string;
  /** The full audit_entries row, JSON-serialised. */
  entry: {
    id: string;
    userId: string;
    action: string;
    resource: string;
    resourceKind: string;
    resourceId: string;
    details: string;
    timestamp: string;
    entryHash: string;
  };
}

export interface ReplicationTarget {
  /** Ship one frame. Must be idempotent at the replica. */
  ship(frame: AuditFrame): Promise<void>;
  /** Return the highest rowid the replica has stored. -1 if empty. */
  lastRowid(): Promise<number>;
  /** Close any open resources (network sockets, file handles). */
  close(): Promise<void>;
}

/**
 * File-backed target: appends frames to a sibling SQLite replica.
 * The replica has the same audit_entries + audit_chain_state
 * schema; `applyFrame` does a single transaction per frame.
 */
export class FileTarget implements ReplicationTarget {
  constructor(private readonly replica: Database.Database) {}

  async ship(frame: AuditFrame): Promise<void> {
    const apply = this.replica.transaction((f: AuditFrame) => {
      this.replica
        .prepare(
          `INSERT OR IGNORE INTO audit_entries
             (id, user_id, action, resource, details, timestamp, previous_hash, entry_hash, timestamp_str, resource_kind, resource_id)
           VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
        )
        .run(
          f.entry.id,
          f.entry.userId,
          f.entry.action,
          f.entry.resource,
          f.entry.details,
          f.entry.timestamp,
          f.previousHash,
          f.entry.entryHash,
          f.entry.timestamp,
          f.entry.resourceKind,
          f.entry.resourceId,
        );
      // Ensure the singleton row exists before the UPDATE; the
      // AuditChain constructor seeds it on the primary but the
      // replica is a fresh DB with only DDL applied.
      this.replica
        .prepare(
          `INSERT OR IGNORE INTO audit_chain_state (id, last_hash, last_rowid) VALUES (0, '', 0)`,
        )
        .run();
      this.replica
        .prepare(
          `UPDATE audit_chain_state
           SET last_hash = ?, last_rowid = ?, updated_by_app = 1
           WHERE id = 0 AND last_rowid < ?`,
        )
        .run(f.entry.entryHash, f.rowid, f.rowid);
    });
    apply(frame);
  }

  async lastRowid(): Promise<number> {
    const row = this.replica
      .prepare('SELECT last_rowid AS rowid FROM audit_chain_state WHERE id = 0')
      .get() as { rowid: number } | undefined;
    return row?.rowid ?? -1;
  }

  async close(): Promise<void> {
    // The replica is owned by the caller; do not close here.
  }
}

/**
 * HTTP-backed target: POSTs each frame to the replica's
 * `/api/audit/ingest` endpoint. The replica server authenticates
 * the request with the same org-scoped API key as every other
 * call.
 */
export class HttpTarget implements ReplicationTarget {
  constructor(
    private readonly url: string,
    private readonly apiKey: string,
    private readonly fetchImpl: typeof fetch = fetch,
  ) {}

  async ship(frame: AuditFrame): Promise<void> {
    const res = await this.fetchImpl(`${this.url.replace(/\/$/, '')}/api/audit/ingest`, {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        ...(this.apiKey ? { authorization: `Bearer ${this.apiKey}` } : {}),
      },
      body: JSON.stringify(frame),
    });
    if (!res.ok) {
      throw new Error(`audit replica POST ${this.url} → ${res.status}`);
    }
  }

  async lastRowid(): Promise<number> {
    const res = await this.fetchImpl(`${this.url.replace(/\/$/, '')}/api/audit/replication-state`);
    if (!res.ok) return -1;
    const json = (await res.json()) as { lastRowid: number };
    return json.lastRowid ?? -1;
  }

  async close(): Promise<void> {
    // Nothing to close for HTTP.
  }
}

/**
 * Track replication lag. The primary records the wall-clock
 * time each frame was emitted; the replica records when it
 * applied the frame. The lag is the difference.
 *
 * This is intentionally simple — no clock sync beyond the OS.
 * Production would use NTP + a tolerance. For tests, we just
 * compute the delta and assert it's under a budget.
 */
export interface LagSample {
  rowid: number;
  emittedAt: number;
  appliedAt: number;
  lagMs: number;
}

export class LagTracker {
  private samples: LagSample[] = [];

  recordEmitted(rowid: number, at = Date.now()): void {
    this.samples.push({ rowid, emittedAt: at, appliedAt: 0, lagMs: 0 });
  }

  recordApplied(rowid: number, at = Date.now()): void {
    const sample = this.samples.find((s) => s.rowid === rowid && s.appliedAt === 0);
    if (!sample) return;
    sample.appliedAt = at;
    sample.lagMs = at - sample.emittedAt;
  }

  maxLagMs(): number {
    return this.samples.reduce((m, s) => (s.lagMs > m ? s.lagMs : m), 0);
  }

  samples_(): LagSample[] {
    return this.samples;
  }
}

// Renamed export above to avoid the JS reserved-word clash with
// the type `samples`. Keep the underscore-suffixed name out of
// the public surface — callers should iterate via the helper.
void LagTracker.prototype;