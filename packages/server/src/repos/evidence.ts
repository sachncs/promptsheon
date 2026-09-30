import { randomUUID } from 'node:crypto';
import type Database from 'better-sqlite3';
import { canonicalTelemetryJson, hashTelemetry, redactTelemetry, type RedactionSensitivity } from '../observability/redaction.js';

export const EVIDENCE_EVENT_TYPES = [
  'execution.started',
  'execution.completed',
  'execution.failed',
  'execution.cancelled',
  'model.called',
  'tool.called',
  'guardrail.decided',
  'permission.decided',
  'resource.consumed',
  'error.observed',
] as const;

export type EvidenceEventType = (typeof EVIDENCE_EVENT_TYPES)[number];

export interface EvidenceRecord {
  id: string;
  eventType: EvidenceEventType;
  schemaVersion: string;
  occurredAt: string;
  organizationId: string;
  workspaceId: string | null;
  correlationId: string;
  traceId: string | null;
  executionId: string | null;
  agentHash: string | null;
  stepId: string | null;
  retentionClass: string;
  payload: unknown;
  payloadHash: string;
  createdAt: string;
}

export interface AppendEvidenceInput {
  eventType: EvidenceEventType;
  occurredAt?: string;
  organizationId: string;
  workspaceId?: string | null;
  correlationId: string;
  traceId?: string | null;
  executionId?: string | null;
  agentHash?: string | null;
  stepId?: string | null;
  retentionClass?: string;
  sensitivity?: RedactionSensitivity;
  payload: unknown;
}

interface EvidenceRow {
  id: string;
  event_type: EvidenceEventType;
  schema_version: string;
  occurred_at: string;
  organization_id: string;
  workspace_id: string | null;
  correlation_id: string;
  trace_id: string | null;
  execution_id: string | null;
  agent_hash: string | null;
  step_id: string | null;
  retention_class: string;
  payload_json: string;
  payload_hash: string;
  created_at: string;
}

function rowToEvidence(row: EvidenceRow): EvidenceRecord {
  return {
    id: row.id,
    eventType: row.event_type,
    schemaVersion: row.schema_version,
    occurredAt: row.occurred_at,
    organizationId: row.organization_id,
    workspaceId: row.workspace_id,
    correlationId: row.correlation_id,
    traceId: row.trace_id,
    executionId: row.execution_id,
    agentHash: row.agent_hash,
    stepId: row.step_id,
    retentionClass: row.retention_class,
    payload: JSON.parse(row.payload_json) as unknown,
    payloadHash: row.payload_hash,
    createdAt: row.created_at,
  };
}

/** Append-only persistence boundary for redacted execution evidence. */
export class EvidenceRepo {
  constructor(private readonly db: Database.Database) {}

  append(input: AppendEvidenceInput): EvidenceRecord {
    const payload = redactTelemetry(input.payload, input.sensitivity ?? 'internal');
    const payloadJson = canonicalTelemetryJson(payload);
    const now = new Date().toISOString();
    const row: EvidenceRow = {
      id: randomUUID(),
      event_type: input.eventType,
      schema_version: '1.0',
      occurred_at: input.occurredAt ?? now,
      organization_id: input.organizationId,
      workspace_id: input.workspaceId ?? null,
      correlation_id: input.correlationId,
      trace_id: input.traceId ?? null,
      execution_id: input.executionId ?? null,
      agent_hash: input.agentHash ?? null,
      step_id: input.stepId ?? null,
      retention_class: input.retentionClass ?? 'execution',
      payload_json: payloadJson,
      payload_hash: hashTelemetry(payload),
      created_at: now,
    };
    this.db.prepare(`
      INSERT INTO evidence_records
        (id, event_type, schema_version, occurred_at, organization_id, correlation_id,
         trace_id, execution_id, agent_hash, step_id, retention_class, payload_json,
         payload_hash, created_at, workspace_id)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `).run(
      row.id, row.event_type, row.schema_version, row.occurred_at, row.organization_id,
      row.correlation_id, row.trace_id, row.execution_id, row.agent_hash, row.step_id,
      row.retention_class, row.payload_json, row.payload_hash, row.created_at,
      row.workspace_id,
    );
    return rowToEvidence(row);
  }

  listByOrganization(organizationId: string, options: { limit?: number; before?: string; eventType?: EvidenceEventType; agentHash?: string; workspaceId?: string } = {}): EvidenceRecord[] {
    const limit = Math.min(options.limit ?? 100, 500);
    const conditions = ['organization_id = ?'];
    const args: unknown[] = [organizationId];
    if (options.before) { conditions.push('occurred_at < ?'); args.push(options.before); }
    if (options.eventType) { conditions.push('event_type = ?'); args.push(options.eventType); }
    if (options.agentHash) { conditions.push('agent_hash = ?'); args.push(options.agentHash); }
    if (options.workspaceId) { conditions.push('workspace_id = ?'); args.push(options.workspaceId); }
    const rows = this.db.prepare(`
      SELECT * FROM evidence_records
      WHERE ${conditions.join(' AND ')}
      ORDER BY occurred_at DESC, id DESC LIMIT ?
    `).all(...args, limit) as EvidenceRow[];
    return rows.map(rowToEvidence);
  }

  listByTrace(organizationId: string, traceId: string, workspaceId?: string): EvidenceRecord[] {
    const workspaceClause = workspaceId ? ' AND workspace_id = ?' : '';
    const rows = this.db.prepare(`
      SELECT * FROM evidence_records
      WHERE organization_id = ? AND trace_id = ?${workspaceClause}
      ORDER BY occurred_at ASC, id ASC
    `).all(...(workspaceId ? [organizationId, traceId, workspaceId] : [organizationId, traceId])) as EvidenceRow[];
    return rows.map(rowToEvidence);
  }

  deleteBefore(organizationId: string, cutoff: string, retentionClass?: string): number {
    const result = retentionClass
      ? this.db.prepare('DELETE FROM evidence_records WHERE organization_id = ? AND occurred_at < ? AND retention_class = ?').run(organizationId, cutoff, retentionClass)
      : this.db.prepare('DELETE FROM evidence_records WHERE organization_id = ? AND occurred_at < ?').run(organizationId, cutoff);
    return result.changes;
  }
}
