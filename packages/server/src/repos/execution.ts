import type { Execution, ExecutionReplay } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { randomUUID } from 'node:crypto';
import { BaseRepo } from './base.js';

export class ExecutionRepo extends BaseRepo<Execution> {
  constructor(db: Database.Database) {
    super(db, 'executions');
  }

  findByVersionId(versionId: string, opts: { page: number; pageSize: number }): { items: Execution[]; total: number } {
    const total = (this.db.prepare('SELECT COUNT(*) as count FROM executions WHERE capability_version_id = ?').get(versionId) as { count: number }).count;
    const items = this.db.prepare('SELECT * FROM executions WHERE capability_version_id = ? ORDER BY timestamp DESC LIMIT ? OFFSET ?')
      .all(versionId, opts.pageSize, (opts.page - 1) * opts.pageSize) as Execution[];
    return { items, total };
  }

  findRecent(capabilityId: string, limit = 100): Execution[] {
    return this.db.prepare(`SELECT e.* FROM executions e JOIN capability_versions v ON e.capability_version_id = v.id WHERE v.capability_id = ? ORDER BY e.timestamp DESC LIMIT ?`)
      .all(capabilityId, limit) as Execution[];
  }

  create(data: {
    capabilityVersionId: string | null;
    inputs: string;
    outputs: string;
    model: string;
    provider: string;
    latencyMs: number;
    costUsd: number;
    promptTokens: number;
    completionTokens: number;
    totalTokens: number;
    error: string;
    traceId: string;
    environment: string;
    replayOf?: string | null;
    inputHash?: string | null;
  }): Execution {
    const id = randomUUID();
    const now = new Date().toISOString();
    this.db.prepare(`INSERT INTO executions (id, capability_version_id, inputs, outputs, model, provider, latency_ms, cost_usd, prompt_tokens, completion_tokens, total_tokens, error, trace_id, environment, timestamp, replay_of, replay_count, input_hash) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?)`)
      .run(
        id,
        data.capabilityVersionId,
        data.inputs,
        data.outputs,
        data.model,
        data.provider,
        data.latencyMs,
        data.costUsd,
        data.promptTokens,
        data.completionTokens,
        data.totalTokens,
        data.error,
        data.traceId,
        data.environment,
        now,
        data.replayOf ?? null,
        data.inputHash ?? null,
      );
    return {
      id, capabilityVersionId: data.capabilityVersionId, inputs: data.inputs, outputs: data.outputs,
      model: data.model, provider: data.provider, latencyMs: data.latencyMs, costUsd: data.costUsd,
      promptTokens: data.promptTokens, completionTokens: data.completionTokens, totalTokens: data.totalTokens,
      error: data.error, traceId: data.traceId, environment: data.environment, timestamp: now,
      replayOf: data.replayOf ?? null, replayCount: 0, inputHash: data.inputHash ?? null,
    };
  }

  /**
   * Update the post-execution fields on a row. Used by the replay
   * service to fill in the final outputs/cost/latency after the
   * executor has run. Returns the post-update row, or null if the
   * row no longer exists.
   */
  updateRunResult(
    id: string,
    fields: {
      outputs: string;
      latencyMs: number;
      costUsd: number;
      totalTokens: number;
      error: string;
    },
  ): Execution | null {
    const result = this.db.prepare(
      `UPDATE executions
       SET outputs = ?, latency_ms = ?, cost_usd = ?, total_tokens = ?, error = ?
       WHERE id = ?`,
    ).run(fields.outputs, fields.latencyMs, fields.costUsd, fields.totalTokens, fields.error, id);
    if (result.changes === 0) return null;
    return this.findById(id);
  }

  /**
   * Resolve everything needed to re-run an execution with its
   * original inputs: the execution row, the manifest it ran against,
   * and the manifest hash. Returns null if any link is broken.
   *
   * `inputs` is parsed from JSON; if it cannot be parsed (legacy
   * rows store a hash here) we throw a deterministic error so the
   * caller can return 409 instead of attempting an undefined replay.
   */
  findReplayContext(id: string): {
    execution: Execution;
    manifestHash: string;
    parsedInputs: Record<string, unknown>;
  } | null {
    const execution = this.findById(id);
    if (!execution) return null;
    if (!execution.capabilityVersionId) return null;
    const row = this.db.prepare(
      `SELECT manifest_hash AS manifestHash
       FROM capability_versions
       WHERE id = ?`,
    ).get(execution.capabilityVersionId) as { manifestHash: string | null } | undefined;
    if (!row?.manifestHash) return null;
    let parsed: Record<string, unknown>;
    try {
      const raw = JSON.parse(execution.inputs) as unknown;
      if (!raw || typeof raw !== 'object' || Array.isArray(raw)) {
        throw new Error('inputs column is not a JSON object');
      }
      parsed = raw as Record<string, unknown>;
    } catch {
      throw new ReplayInputsUnavailableError(id);
    }
    return { execution, manifestHash: row.manifestHash, parsedInputs: parsed };
  }

  /**
   * Increment `replay_count` on the original execution. Returns the
   * new count, or null if the execution no longer exists.
   */
  incrementReplayCount(id: string): number | null {
    const result = this.db.prepare(
      `UPDATE executions SET replay_count = replay_count + 1 WHERE id = ?`,
    ).run(id);
    if (result.changes === 0) return null;
    const row = this.db.prepare(
      `SELECT replay_count AS replayCount FROM executions WHERE id = ?`,
    ).get(id) as { replayCount: number } | undefined;
    return row?.replayCount ?? null;
  }

  recordReplay(data: {
    originalExecutionId: string;
    replayExecutionId: string | null;
    outcome: ExecutionReplay['outcome'];
    inputsMatch: boolean;
    manifestMatch: boolean;
    modelMatch: boolean;
    environmentMatch: boolean;
    diffSummary: string | null;
  }): ExecutionReplay {
    const id = randomUUID();
    const now = new Date().toISOString();
    this.db.prepare(`INSERT INTO execution_replays (id, original_execution_id, replay_execution_id, outcome, inputs_match, manifest_match, model_match, environment_match, diff_summary, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
      .run(
        id,
        data.originalExecutionId,
        data.replayExecutionId,
        data.outcome,
        data.inputsMatch ? 1 : 0,
        data.manifestMatch ? 1 : 0,
        data.modelMatch ? 1 : 0,
        data.environmentMatch ? 1 : 0,
        data.diffSummary,
        now,
      );
    return {
      id,
      originalExecutionId: data.originalExecutionId,
      replayExecutionId: data.replayExecutionId,
      outcome: data.outcome,
      inputsMatch: data.inputsMatch,
      manifestMatch: data.manifestMatch,
      modelMatch: data.modelMatch,
      environmentMatch: data.environmentMatch,
      diffSummary: data.diffSummary,
      createdAt: now,
    };
  }

  findReplaysByOriginal(originalId: string, limit = 100): ExecutionReplay[] {
    const rows = this.db.prepare(
      `SELECT * FROM execution_replays WHERE original_execution_id = ? ORDER BY created_at DESC, id DESC LIMIT ?`,
    ).all(originalId, limit) as Array<Record<string, unknown>>;
    return rows.map((r) => replayRowToObject(r));
  }
}

/**
 * Distinct error type so the replay route can map to HTTP 409
 * without sniffing message strings.
 */
export class ReplayInputsUnavailableError extends Error {
  constructor(public readonly executionId: string) {
    super(
      `execution ${executionId} cannot be replayed: inputs were not captured ` +
      `(pre-migration 049 rows store a hash instead of JSON)`,
    );
    this.name = 'ReplayInputsUnavailableError';
  }
}

function replayRowToObject(row: Record<string, unknown>): ExecutionReplay {
  return {
    id: row['id'] as string,
    originalExecutionId: row['original_execution_id'] as string,
    replayExecutionId: (row['replay_execution_id'] as string | null) ?? null,
    outcome: row['outcome'] as ExecutionReplay['outcome'],
    inputsMatch: ((row['inputs_match'] as number) ?? 0) === 1,
    manifestMatch: ((row['manifest_match'] as number) ?? 0) === 1,
    modelMatch: ((row['model_match'] as number) ?? 0) === 1,
    environmentMatch: ((row['environment_match'] as number) ?? 0) === 1,
    diffSummary: (row['diff_summary'] as string | null) ?? null,
    createdAt: row['created_at'] as string,
  };
}