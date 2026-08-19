import type { Execution } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
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

  create(data: { capabilityVersionId: string | null; inputs: string; outputs: string; model: string; provider: string; latencyMs: number; costUsd: number; promptTokens: number; completionTokens: number; totalTokens: number; error: string; traceId: string; environment: string }): Execution {
    const id = crypto.randomUUID();
    const now = new Date().toISOString();
    this.db.prepare(`INSERT INTO executions (id, capability_version_id, inputs, outputs, model, provider, latency_ms, cost_usd, prompt_tokens, completion_tokens, total_tokens, error, trace_id, environment, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
      .run(id, data.capabilityVersionId, data.inputs, data.outputs, data.model, data.provider, data.latencyMs, data.costUsd, data.promptTokens, data.completionTokens, data.totalTokens, data.error, data.traceId, data.environment, now);
    return {
      id, capabilityVersionId: data.capabilityVersionId, inputs: data.inputs, outputs: data.outputs,
      model: data.model, provider: data.provider, latencyMs: data.latencyMs, costUsd: data.costUsd,
      promptTokens: data.promptTokens, completionTokens: data.completionTokens, totalTokens: data.totalTokens,
      error: data.error, traceId: data.traceId, environment: data.environment, timestamp: now,
    };
  }
}