import type { Execution, ExecutionReplay } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { randomUUID } from 'node:crypto';
import { z } from 'zod';
import { BaseRepo } from './base.js';

const ExecutionRowSchema = z.object({
  id: z.string(),
  capability_version_id: z.string().nullable(),
  timestamp: z.string(),
  inputs: z.string(),
  outputs: z.string(),
  model: z.string(),
  provider: z.string(),
  latency_ms: z.number().int().nonnegative(),
  cost_usd: z.number().nonnegative(),
  prompt_tokens: z.number().int().nonnegative(),
  completion_tokens: z.number().int().nonnegative(),
  total_tokens: z.number().int().nonnegative(),
  error: z.string(),
  trace_id: z.string(),
  environment: z.string(),
  replay_of: z.string().nullable(),
  replay_count: z.number().int().nonnegative(),
  input_hash: z.string().nullable(),
});

const CountSchema = z.object({ count: z.number().int().nonnegative() });

function toExecution(row: unknown): Execution {
  const value = ExecutionRowSchema.parse(row);
  return {
    id: value.id,
    capabilityVersionId: value.capability_version_id,
    timestamp: value.timestamp,
    inputs: value.inputs,
    outputs: value.outputs,
    model: value.model,
    provider: value.provider,
    latencyMs: value.latency_ms,
    costUsd: value.cost_usd,
    promptTokens: value.prompt_tokens,
    completionTokens: value.completion_tokens,
    totalTokens: value.total_tokens,
    error: value.error,
    traceId: value.trace_id,
    environment: value.environment,
    replayOf: value.replay_of,
    replayCount: value.replay_count,
    inputHash: value.input_hash,
  };
}

export class ExecutionRepo extends BaseRepo<Execution> {
  constructor(db: Database.Database) {
    super(db, 'executions');
  }

  findByVersionId(versionId: string, opts: { page: number; pageSize: number }): { items: Execution[]; total: number } {
    const total = CountSchema.parse(
      this.db.prepare('SELECT COUNT(*) as count FROM executions WHERE capability_version_id = ?').get(versionId),
    ).count;
    const items = this.db.prepare('SELECT * FROM executions WHERE capability_version_id = ? ORDER BY timestamp DESC LIMIT ? OFFSET ?')
      .all(versionId, opts.pageSize, (opts.page - 1) * opts.pageSize)
      .map(toExecution);
    return { items, total };
  }

  findByVersionIdInOrg(versionId: string, organizationId: string, opts: { page: number; pageSize: number }): { items: Execution[]; total: number } {
    const scope = `FROM executions e
      JOIN capability_versions v ON v.id = e.capability_version_id
      JOIN capabilities c ON c.id = v.capability_id
      JOIN projects p ON p.id = c.project_id
      JOIN workspaces w ON w.id = p.workspace_id
      WHERE v.id = ? AND w.org_id = ?`;
    const total = CountSchema.parse(this.db.prepare(`SELECT COUNT(*) AS count ${scope}`).get(versionId, organizationId)).count;
    const items = this.db.prepare(`SELECT e.* ${scope} ORDER BY e.timestamp DESC LIMIT ? OFFSET ?`)
      .all(versionId, organizationId, opts.pageSize, (opts.page - 1) * opts.pageSize)
      .map(toExecution);
    return { items, total };
  }

  findByIdInOrg(id: string, organizationId: string): Execution | null {
    const row = this.db.prepare(
      `SELECT e.* FROM executions e
       JOIN capability_versions v ON v.id = e.capability_version_id
       JOIN capabilities c ON c.id = v.capability_id
       JOIN projects p ON p.id = c.project_id
       JOIN workspaces w ON w.id = p.workspace_id
       WHERE e.id = ? AND w.org_id = ?`,
    ).get(id, organizationId);
    return row ? toExecution(row) : null;
  }

  findManyInOrg(organizationId: string, opts: { page: number; pageSize: number }): { items: Execution[]; total: number } {
    const scope = `FROM executions e
      JOIN capability_versions v ON v.id = e.capability_version_id
      JOIN capabilities c ON c.id = v.capability_id
      JOIN projects p ON p.id = c.project_id
      JOIN workspaces w ON w.id = p.workspace_id
      WHERE w.org_id = ?`;
    const total = CountSchema.parse(this.db.prepare(`SELECT COUNT(*) AS count ${scope}`).get(organizationId)).count;
    const items = this.db.prepare(`SELECT e.* ${scope} ORDER BY e.timestamp DESC LIMIT ? OFFSET ?`)
      .all(organizationId, opts.pageSize, (opts.page - 1) * opts.pageSize)
      .map(toExecution);
    return { items, total };
  }

  findRecent(capabilityId: string, limit = 100): Execution[] {
    return this.db.prepare(`SELECT e.* FROM executions e JOIN capability_versions v ON e.capability_version_id = v.id WHERE v.capability_id = ? ORDER BY e.timestamp DESC LIMIT ?`)
      .all(capabilityId, limit)
      .map(toExecution);
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
  findReplayContextInOrg(id: string, organizationId: string): {
    execution: Execution;
    manifestHash: string;
    parsedInputs: Record<string, unknown>;
  } | null {
    const row = this.db.prepare(
      `SELECT e.*, v.manifest_hash AS manifestHash
       FROM executions e
       JOIN capability_versions v ON v.id = e.capability_version_id
       JOIN capabilities c ON c.id = v.capability_id
       JOIN projects p ON p.id = c.project_id
       JOIN workspaces w ON w.id = p.workspace_id
       WHERE e.id = ? AND w.org_id = ?`,
    ).get(id, organizationId);
    if (!row) return null;
    const parsedRow = ExecutionRowSchema.extend({ manifestHash: z.string().nullable() }).parse(row);
    if (!parsedRow.manifestHash) return null;
    const execution = toExecution(parsedRow);
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
    return { execution, manifestHash: parsedRow.manifestHash, parsedInputs: parsed };
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
    const row = z.object({ replayCount: z.number().int().nonnegative() }).safeParse(
      this.db.prepare(
        `SELECT replay_count AS replayCount FROM executions WHERE id = ?`,
      ).get(id),
    );
    return row.success ? row.data.replayCount : null;
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
    ).all(originalId, limit);
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

const ExecutionReplayRowSchema = z.object({
  id: z.string(),
  original_execution_id: z.string(),
  replay_execution_id: z.string().nullable(),
  outcome: z.enum(['started', 'completed', 'diverged', 'failed']),
  inputs_match: z.union([z.number().int(), z.boolean()]),
  manifest_match: z.union([z.number().int(), z.boolean()]),
  model_match: z.union([z.number().int(), z.boolean()]),
  environment_match: z.union([z.number().int(), z.boolean()]),
  diff_summary: z.string().nullable(),
  created_at: z.string(),
});

function replayRowToObject(row: unknown): ExecutionReplay {
  const value = ExecutionReplayRowSchema.parse(row);
  return {
    id: value.id,
    originalExecutionId: value.original_execution_id,
    replayExecutionId: value.replay_execution_id,
    outcome: value.outcome,
    inputsMatch: value.inputs_match === true || value.inputs_match === 1,
    manifestMatch: value.manifest_match === true || value.manifest_match === 1,
    modelMatch: value.model_match === true || value.model_match === 1,
    environmentMatch: value.environment_match === true || value.environment_match === 1,
    diffSummary: value.diff_summary,
    createdAt: value.created_at,
  };
}
