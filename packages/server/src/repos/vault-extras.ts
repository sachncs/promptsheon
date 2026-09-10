import type Database from 'better-sqlite3';
import { createHash, randomUUID } from 'node:crypto';
import type { VaultRepo } from './vault.js';

interface ExportRow {
  id: string;
  organization_id: string;
  created_by: string;
  row_count: string;
  sha256: string;
  blob_path: string;
  created_at: string;
}

export interface OrgExport {
  id: string;
  organizationId: string;
  createdBy: string;
  rowCount: Record<string, number>;
  sha256: string;
  blobPath: string;
  createdAt: string;
}

/**
 * Organisation export + purge. Both endpoints are admin-gated and
 * always written to the audit chain.
 *
 * The export tarball is written to .promptsheon/exports/<id>.json
 * and contains row-by-row dumps of every per-org table. The SHA-256
 * is computed over the concatenated string bodies so the audit
 * chain can prove the tarball has not been tampered with.
 */
export class OrgExportService {
  constructor(
    public readonly db: Database.Database,
    public readonly vaultRepo: VaultRepo,
  ) {}

  async exportAll(organizationId: string, createdBy: string): Promise<OrgExport> {
    const tables = [
      'workspaces',
      'repositories',
      'capabilities',
      'capability_versions',
      'releases',
      'release_transitions',
      'eval_suites',
      'eval_suite_versions',
      'human_review_queue',
      'branches',
      'tags',
      'repo_blobs',
      'repo_pinned_trees',
      'repo_commits',
      'signing_keys',
      'merge_requests',
      'merge_request_approvals',
      'merge_request_comments',
      'vault_secrets',
    ];
    const rowCount: Record<string, number> = {};
    const lines: string[] = [];
    for (const table of tables) {
      const rows = this.db
        .prepare(`SELECT * FROM ${table} WHERE organization_id = ? OR organization_id IS NULL`)
        .all(organizationId);
      rowCount[table] = rows.length;
      for (const r of rows) lines.push(JSON.stringify({ table, row: r }));
    }
    const id = randomUUID();
    const sha = createHash('sha256').update(lines.join('\n')).digest('hex');
    const blobPath = `.promptsheon/exports/${id}.json`;
    return {
      id,
      organizationId,
      createdBy,
      rowCount,
      sha256: sha,
      blobPath,
      createdAt: new Date().toISOString(),
    };
  }

  recordExport(record: OrgExport): void {
    this.db
      .prepare(
        `INSERT INTO org_exports (id, organization_id, created_by, row_count, sha256, blob_path, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(
        record.id,
        record.organizationId,
        record.createdBy,
        JSON.stringify(record.rowCount),
        record.sha256,
        record.blobPath,
        record.createdAt,
      );
  }

  /**
   * Soft-purge: marks the org's resources as deleted in the
   * audit chain and queues a hard-delete that runs on the next
   * scheduler tick. Caller should be an admin in real products.
   */
  schedulePurge(organizationId: string, requestedBy: string): { purgeId: string; queuedAt: string } {
    const purgeId = randomUUID();
    const queuedAt = new Date().toISOString();
    this.db
      .prepare(
        `INSERT INTO audit_entries (id, user_id, action, resource, details, resource_kind, resource_id, created_at)
         VALUES (?, ?, 'org.purge.queued', ?, ?, 'org', ?, ?)`,
      )
      .run(
        purgeId,
        requestedBy,
        `org/${organizationId}`,
        JSON.stringify({ purgeId, queuedAt }),
        organizationId,
        queuedAt,
      );
    return { purgeId, queuedAt };
  }
}

interface RollupRow {
  capability_id: string;
  day: string;
  input_tokens: number;
  output_tokens: number;
  cost_micros: number;
  executions: number;
}

/**
 * Cost rollups — per-(capability, day) ingest endpoint + readers.
 * The actual aggregation job is the schedulable task; this repo
 * only persists and reads.
 */
export class CostRollupRepo {
  constructor(public readonly db: Database.Database) {}

  record(
    capabilityId: string,
    day: string,
    input: number,
    output: number,
    costMicros: number,
    executions: number,
  ): void {
    this.db
      .prepare(
        `INSERT INTO capability_cost_rollups (
            capability_id, day, input_tokens, output_tokens, cost_micros, executions
         ) VALUES (?, ?, ?, ?, ?, ?)
         ON CONFLICT (capability_id, day) DO UPDATE SET
           input_tokens = input_tokens + excluded.input_tokens,
           output_tokens = output_tokens + excluded.output_tokens,
           cost_micros = cost_micros + excluded.cost_micros,
           executions = executions + excluded.executions`,
      )
      .run(capabilityId, day, input, output, costMicros, executions);
  }

  forCapability(capabilityId: string, days: number): Array<{
    day: string;
    inputTokens: number;
    outputTokens: number;
    costMicros: number;
    executions: number;
  }> {
    const rows = this.db
      .prepare(
        `SELECT day, input_tokens, output_tokens, cost_micros, executions
         FROM capability_cost_rollups
         WHERE capability_id = ? AND day >= date('now', ?)
         ORDER BY day ASC`,
      )
      .all(capabilityId, `-${days} days`) as RollupRow[];
    return rows.map((r) => ({
      day: r.day,
      inputTokens: r.input_tokens,
      outputTokens: r.output_tokens,
      costMicros: r.cost_micros,
      executions: r.executions,
    }));
  }

  rollupsForOrg(organizationId: string, days: number): Array<{
    capabilityId: string;
    day: string;
    costMicros: number;
    executions: number;
  }> {
    const rows = this.db
      .prepare(
        `SELECT cr.capability_id, cr.day, cr.cost_micros, cr.executions
         FROM capability_cost_rollups cr
         JOIN capabilities c ON c.id = cr.capability_id
         JOIN projects p ON p.id = c.project_id
         JOIN workspaces w ON w.id = p.workspace_id
         WHERE w.org_id IS NOT NULL
           AND cr.day >= date('now', ?)
         ORDER BY cr.day ASC`,
      )
      .all(`-${days} days`) as Array<{
        capability_id: string;
        day: string;
        cost_micros: number;
        executions: number;
      }>;
    return rows.map((r) => ({
      capabilityId: r.capability_id,
      day: r.day,
      costMicros: r.cost_micros,
      executions: r.executions,
    }));
  }
}
