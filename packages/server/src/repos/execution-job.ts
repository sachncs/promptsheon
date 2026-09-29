import type Database from 'better-sqlite3';
import { randomUUID } from 'node:crypto';
import { assertExecutionTransition, type ExecutionState } from '@promptsheon/shared';
import { NotFoundError } from '@promptsheon/shared';

export interface ExecutionJob {
  id: string;
  organizationId: string;
  workspaceId: string;
  agentHash: string;
  inputHash: string;
  inputJson: string;
  idempotencyKey: string;
  state: ExecutionState;
  attempts: number;
  maxAttempts: number;
  availableAt: string;
  leaseOwner: string | null;
  leaseExpiresAt: string | null;
  resultJson: string | null;
  error: string | null;
  createdAt: string;
  startedAt: string | null;
  completedAt: string | null;
}

export interface ExecutionQueueMetrics {
  queued: number;
  running: number;
  completed: number;
  failed: number;
  cancelled: number;
  timedOut: number;
  partiallyCompleted: number;
  oldestQueuedAt: string | null;
}

export class IdempotencyConflictError extends Error {
  readonly statusCode = 409;

  constructor(key: string) {
    super(`idempotency key already belongs to a different execution request: ${key}`);
    this.name = 'IdempotencyConflictError';
  }
}

export class ExecutionQueueCapacityError extends Error {
  readonly statusCode = 429;

  constructor(limit: number) {
    super(`execution queue capacity reached (${limit} pending jobs)`);
    this.name = 'ExecutionQueueCapacityError';
  }
}

interface JobRow {
  id: string;
  organization_id: string;
  workspace_id: string;
  agent_hash: string;
  input_hash: string;
  input_json: string;
  idempotency_key: string;
  state: ExecutionState;
  attempts: number;
  max_attempts: number;
  available_at: string;
  lease_owner: string | null;
  lease_expires_at: string | null;
  result_json: string | null;
  error: string | null;
  created_at: string;
  started_at: string | null;
  completed_at: string | null;
}

/** SQLite-backed durable queue metadata and idempotent lifecycle transitions. */
export class ExecutionJobRepo {
  constructor(private readonly db: Database.Database, private readonly maxPendingJobsPerOrganization = 10_000) {}

  enqueue(input: {
    organizationId: string;
    workspaceId: string;
    agentHash: string;
    inputHash: string;
    inputJson: string;
    idempotencyKey: string;
    maxAttempts?: number;
  }): ExecutionJob {
    const existing = this.db.prepare(
      'SELECT * FROM execution_jobs WHERE organization_id = ? AND idempotency_key = ?',
    ).get(input.organizationId, input.idempotencyKey) as JobRow | undefined;
    if (existing) {
      if (existing.agent_hash !== input.agentHash || existing.input_hash !== input.inputHash) {
        throw new IdempotencyConflictError(input.idempotencyKey);
      }
      return toJob(existing);
    }
    const pending = this.db.prepare(
      "SELECT COUNT(*) AS count FROM execution_jobs WHERE organization_id = ? AND state IN ('queued', 'running')",
    ).get(input.organizationId) as { count: number };
    if (pending.count >= this.maxPendingJobsPerOrganization) throw new ExecutionQueueCapacityError(this.maxPendingJobsPerOrganization);
    const now = new Date().toISOString();
    const id = randomUUID();
    this.db.prepare(`
      INSERT INTO execution_jobs
        (id, organization_id, workspace_id, agent_hash, input_hash, input_json, idempotency_key, state, attempts, max_attempts, available_at, created_at)
      VALUES (?, ?, ?, ?, ?, ?, ?, 'queued', 0, ?, ?, ?)
    `).run(id, input.organizationId, input.workspaceId, input.agentHash, input.inputHash, input.inputJson,
      input.idempotencyKey, input.maxAttempts ?? 3, now, now);
    return this.get(input.organizationId, id);
  }

  get(organizationId: string, id: string): ExecutionJob {
    const row = this.db.prepare(
      'SELECT * FROM execution_jobs WHERE organization_id = ? AND id = ?',
    ).get(organizationId, id) as JobRow | undefined;
    if (!row) throw new NotFoundError('execution job', id);
    return toJob(row);
  }

  metrics(organizationId: string): ExecutionQueueMetrics {
    const rows = this.db.prepare(
      'SELECT state, COUNT(*) AS count FROM execution_jobs WHERE organization_id = ? GROUP BY state',
    ).all(organizationId) as Array<{ state: ExecutionState; count: number }>;
    const counts = new Map(rows.map((row) => [row.state, row.count]));
    const oldest = this.db.prepare(
      "SELECT MIN(created_at) AS created_at FROM execution_jobs WHERE organization_id = ? AND state = 'queued'",
    ).get(organizationId) as { created_at: string | null };
    return {
      queued: counts.get('queued') ?? 0,
      running: counts.get('running') ?? 0,
      completed: counts.get('completed') ?? 0,
      failed: counts.get('failed') ?? 0,
      cancelled: counts.get('cancelled') ?? 0,
      timedOut: counts.get('timed-out') ?? 0,
      partiallyCompleted: counts.get('partially-completed') ?? 0,
      oldestQueuedAt: oldest.created_at,
    };
  }

  claimNext(workerId: string, leaseMs: number, excludedOrganizations: readonly string[] = []): ExecutionJob | null {
    const now = new Date();
    const nowIso = now.toISOString();
    const leaseUntil = new Date(now.getTime() + leaseMs).toISOString();
    return this.db.transaction(() => {
      const exclusion = excludedOrganizations.length > 0
        ? ` AND organization_id NOT IN (${excludedOrganizations.map(() => '?').join(', ')})`
        : '';
      const row = this.db.prepare(`
        SELECT * FROM execution_jobs
        WHERE ((state = 'queued' AND available_at <= ?)
           OR (state = 'running' AND lease_expires_at IS NOT NULL AND lease_expires_at <= ?))
          ${exclusion}
        ORDER BY created_at ASC
        LIMIT 1
      `).get(nowIso, nowIso, ...excludedOrganizations) as JobRow | undefined;
      if (!row) return null;
      const result = this.db.prepare(`
        UPDATE execution_jobs
        SET state = 'running', attempts = attempts + 1, lease_owner = ?, lease_expires_at = ?, started_at = COALESCE(started_at, ?), error = NULL
        WHERE id = ? AND state = ?
      `).run(workerId, leaseUntil, nowIso, row.id, row.state);
      return result.changes === 1 ? this.get(row.organization_id, row.id) : null;
    })();
  }

  transition(organizationId: string, id: string, from: ExecutionState, to: ExecutionState, fields: {
    resultJson?: string | null;
    error?: string | null;
    availableAt?: string;
    leaseOwner?: string | null;
    leaseExpiresAt?: string | null;
  } = {}): ExecutionJob {
    assertExecutionTransition(from, to);
    const now = new Date().toISOString();
    const result = this.db.prepare(`
      UPDATE execution_jobs
      SET state = ?, result_json = COALESCE(?, result_json), error = ?, available_at = COALESCE(?, available_at),
          lease_owner = ?, lease_expires_at = ?, completed_at = CASE WHEN ? IN ('completed', 'failed', 'cancelled', 'timed-out') THEN ? ELSE completed_at END
      WHERE organization_id = ? AND id = ? AND state = ?
    `).run(to, fields.resultJson ?? null, fields.error ?? null, fields.availableAt ?? null,
      fields.leaseOwner ?? null, fields.leaseExpiresAt ?? null, to, now, organizationId, id, from);
    if (result.changes === 0) throw new Error(`execution ${id} was not in expected state ${from}`);
    return this.get(organizationId, id);
  }

  releaseClaim(organizationId: string, id: string, availableAt: string): ExecutionJob {
    const result = this.db.prepare(`
      UPDATE execution_jobs
      SET state = 'queued', attempts = MAX(0, attempts - 1), available_at = ?, lease_owner = NULL, lease_expires_at = NULL
      WHERE organization_id = ? AND id = ? AND state = 'running'
    `).run(availableAt, organizationId, id);
    if (result.changes === 0) throw new Error(`execution ${id} is not claimed by a worker`);
    return this.get(organizationId, id);
  }

  cancel(organizationId: string, id: string): ExecutionJob {
    const current = this.get(organizationId, id);
    if (['cancelled', 'completed', 'failed', 'timed-out'].includes(current.state)) return current;
    return this.transition(organizationId, id, current.state, 'cancelled');
  }

  requeueExpired(now = new Date()): number {
    const requeued = this.db.prepare(`
      UPDATE execution_jobs
      SET state = 'queued', available_at = ?, lease_owner = NULL, lease_expires_at = NULL, error = 'worker lease expired'
      WHERE state = 'running' AND lease_expires_at IS NOT NULL AND lease_expires_at <= ? AND attempts < max_attempts
    `).run(now.toISOString(), now.toISOString()).changes;
    this.db.prepare(`
      UPDATE execution_jobs
      SET state = 'failed', completed_at = ?, lease_owner = NULL, lease_expires_at = NULL, error = 'worker lease expired after maximum attempts'
      WHERE state = 'running' AND lease_expires_at IS NOT NULL AND lease_expires_at <= ? AND attempts >= max_attempts
    `).run(now.toISOString(), now.toISOString());
    return requeued;
  }
}

function toJob(row: JobRow): ExecutionJob {
  return {
    id: row.id,
    organizationId: row.organization_id,
    workspaceId: row.workspace_id,
    agentHash: row.agent_hash,
    inputHash: row.input_hash,
    inputJson: row.input_json,
    idempotencyKey: row.idempotency_key,
    state: row.state,
    attempts: row.attempts,
    maxAttempts: row.max_attempts,
    availableAt: row.available_at,
    leaseOwner: row.lease_owner,
    leaseExpiresAt: row.lease_expires_at,
    resultJson: row.result_json,
    error: row.error,
    createdAt: row.created_at,
    startedAt: row.started_at,
    completedAt: row.completed_at,
  };
}
