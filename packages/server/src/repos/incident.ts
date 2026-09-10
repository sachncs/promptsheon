import type Database from 'better-sqlite3';
import { randomUUID } from 'node:crypto';

export type IncidentStatus = 'open' | 'accepted' | 'rejected';

export interface IncidentProposal {
  id: string;
  suiteId: string;
  caseId: string;
  sourceKind: 'execution_failure' | 'manual';
  sourceRef: string | null;
  inputText: string;
  expectedText: string;
  severity: 'low' | 'medium' | 'high';
  status: IncidentStatus;
  proposedBy: string;
  reviewedBy: string | null;
  reviewedAt: string | null;
  notes: string | null;
  createdAt: string;
}

interface Row {
  id: string;
  suite_id: string;
  case_id: string;
  source_kind: 'execution_failure' | 'manual';
  source_ref: string | null;
  input_text: string;
  expected_text: string;
  severity: 'low' | 'medium' | 'high';
  status: IncidentStatus;
  proposed_by: string;
  reviewed_by: string | null;
  reviewed_at: string | null;
  notes: string | null;
  created_at: string;
}

function toProp(r: Row): IncidentProposal {
  return {
    id: r.id,
    suiteId: r.suite_id,
    caseId: r.case_id,
    sourceKind: r.source_kind,
    sourceRef: r.source_ref,
    inputText: r.input_text,
    expectedText: r.expected_text,
    severity: r.severity,
    status: r.status,
    proposedBy: r.proposed_by,
    reviewedBy: r.reviewed_by,
    reviewedAt: r.reviewed_at,
    notes: r.notes,
    createdAt: r.created_at,
  };
}

export class IncidentRepo {
  constructor(private db: Database.Database) {}

  list(filter?: { suiteId?: string; status?: IncidentStatus }): IncidentProposal[] {
    let where = '1=1';
    const params: unknown[] = [];
    if (filter?.suiteId) {
      where += ' AND suite_id = ?';
      params.push(filter.suiteId);
    }
    if (filter?.status) {
      where += ' AND status = ?';
      params.push(filter.status);
    }
    const rows = this.db
      .prepare(`SELECT * FROM eval_incident_proposals WHERE ${where} ORDER BY created_at DESC`)
      .all(...params) as Row[];
    return rows.map(toProp);
  }

  findById(id: string): IncidentProposal | null {
    const row = this.db
      .prepare('SELECT * FROM eval_incident_proposals WHERE id = ?')
      .get(id) as Row | undefined;
    return row ? toProp(row) : null;
  }

  create(input: {
    suiteId: string;
    caseId: string;
    sourceKind: 'execution_failure' | 'manual';
    sourceRef: string | null;
    inputText: string;
    expectedText: string;
    severity: 'low' | 'medium' | 'high';
    proposedBy: string;
    notes?: string;
  }): IncidentProposal {
    const id = randomUUID();
    this.db
      .prepare(
        `INSERT INTO eval_incident_proposals (id, suite_id, case_id, source_kind, source_ref,
            input_text, expected_text, severity, status, proposed_by, notes)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'open', ?, ?)`,
      )
      .run(
        id,
        input.suiteId,
        input.caseId,
        input.sourceKind,
        input.sourceRef,
        input.inputText,
        input.expectedText,
        input.severity,
        input.proposedBy,
        input.notes ?? null,
      );
    return this.findById(id)!;
  }

  decide(
    id: string,
    reviewerId: string,
    status: 'accepted' | 'rejected',
    notes: string | null,
  ): IncidentProposal | null {
    this.db
      .prepare(
        `UPDATE eval_incident_proposals
         SET status = ?, reviewed_by = ?, reviewed_at = CURRENT_TIMESTAMP, notes = ?
         WHERE id = ?`,
      )
      .run(status, reviewerId, notes, id);
    return this.findById(id);
  }
}
