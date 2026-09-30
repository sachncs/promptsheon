import type Database from 'better-sqlite3';
import { randomUUID } from 'node:crypto';
import type {
  MutationAuthorType,
  MutationKind,
  MutationProposal,
  MutationProposalStatus,
  MutationRisk,
} from '@promptsheon/shared';

interface MutationProposalRow {
  id: string;
  organization_id: string;
  source_hash: string;
  candidate_hash: string | null;
  mutation_kind: MutationKind;
  changes_json: string;
  rationale: string;
  expected_outcome: string;
  author_type: MutationAuthorType;
  author_id: string;
  risk: MutationRisk;
  confidence: number;
  status: MutationProposalStatus;
  evaluation_run_id: string | null;
  decision_reason: string | null;
  reviewed_by: string | null;
  reviewed_at: string | null;
  created_at: string;
  updated_at: string;
  promoted_release_id: string | null;
  promoted_at: string | null;
}

function toProposal(row: MutationProposalRow): MutationProposal {
  return {
    id: row.id,
    organizationId: row.organization_id,
    sourceHash: row.source_hash,
    candidateHash: row.candidate_hash,
    mutationKind: row.mutation_kind,
    changes: JSON.parse(row.changes_json) as Record<string, unknown>,
    rationale: row.rationale,
    expectedOutcome: row.expected_outcome,
    authorType: row.author_type,
    authorId: row.author_id,
    risk: row.risk,
    confidence: row.confidence,
    status: row.status,
    evaluationRunId: row.evaluation_run_id,
    decisionReason: row.decision_reason,
    reviewedBy: row.reviewed_by,
    reviewedAt: row.reviewed_at,
    createdAt: row.created_at,
    updatedAt: row.updated_at,
    promotedReleaseId: row.promoted_release_id,
    promotedAt: row.promoted_at,
  };
}

export class MutationProposalRepo {
  constructor(private readonly db: Database.Database) {}

  listInOrg(organizationId: string, filter: { status?: MutationProposalStatus; sourceHash?: string } = {}): MutationProposal[] {
    const clauses = ['organization_id = ?'];
    const params: unknown[] = [organizationId];
    if (filter.status) {
      clauses.push('status = ?');
      params.push(filter.status);
    }
    if (filter.sourceHash) {
      clauses.push('source_hash = ?');
      params.push(filter.sourceHash);
    }
    const rows = this.db
      .prepare(`SELECT * FROM mutation_proposals WHERE ${clauses.join(' AND ')} ORDER BY created_at DESC`)
      .all(...params) as MutationProposalRow[];
    return rows.map(toProposal);
  }

  findInOrg(id: string, organizationId: string): MutationProposal | null {
    const row = this.db
      .prepare('SELECT * FROM mutation_proposals WHERE id = ? AND organization_id = ?')
      .get(id, organizationId) as MutationProposalRow | undefined;
    return row ? toProposal(row) : null;
  }

  create(input: {
    organizationId: string;
    sourceHash: string;
    candidateHash?: string;
    mutationKind: MutationKind;
    changes: Record<string, unknown>;
    rationale: string;
    expectedOutcome: string;
    authorType: MutationAuthorType;
    authorId: string;
    risk: MutationRisk;
    confidence: number;
    evaluationRunId?: string;
  }): MutationProposal {
    const id = randomUUID();
    this.db.prepare(
      `INSERT INTO mutation_proposals
       (id, organization_id, source_hash, candidate_hash, mutation_kind, changes_json,
        rationale, expected_outcome, author_type, author_id, risk, confidence, evaluation_run_id)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    ).run(
      id,
      input.organizationId,
      input.sourceHash,
      input.candidateHash ?? null,
      input.mutationKind,
      JSON.stringify(input.changes),
      input.rationale,
      input.expectedOutcome,
      input.authorType,
      input.authorId,
      input.risk,
      input.confidence,
      input.evaluationRunId ?? null,
    );
    return this.findInOrg(id, input.organizationId)!;
  }

  decide(input: {
    id: string;
    organizationId: string;
    status: Extract<MutationProposalStatus, 'approved' | 'rejected' | 'abandoned'>;
    reviewerId: string;
    reason: string;
  }): MutationProposal | null {
    const candidateRequirement = input.status === 'approved' ? " AND status = 'validated' AND candidate_hash IS NOT NULL" : " AND status IN ('proposed', 'validated')";
    const result = this.db.prepare(
      `UPDATE mutation_proposals
       SET status = ?, reviewed_by = ?, reviewed_at = CURRENT_TIMESTAMP, decision_reason = ?
       WHERE id = ? AND organization_id = ?${candidateRequirement}`,
    ).run(input.status, input.reviewerId, input.reason, input.id, input.organizationId);
    if (result.changes === 0) return null;
    return this.findInOrg(input.id, input.organizationId);
  }

  markValidated(id: string, organizationId: string): MutationProposal | null {
    const result = this.db.prepare(
      `UPDATE mutation_proposals
       SET status = 'validated', updated_at = CURRENT_TIMESTAMP
       WHERE id = ? AND organization_id = ? AND status = 'proposed' AND candidate_hash IS NOT NULL`,
    ).run(id, organizationId);
    if (result.changes === 0) return this.findInOrg(id, organizationId);
    return this.findInOrg(id, organizationId);
  }

  markPromoted(id: string, organizationId: string, releaseId: string): MutationProposal | null {
    const result = this.db.prepare(
      `UPDATE mutation_proposals
       SET promoted_release_id = ?, promoted_at = CURRENT_TIMESTAMP
       WHERE id = ? AND organization_id = ? AND status = 'approved' AND promoted_release_id IS NULL`,
    ).run(releaseId, id, organizationId);
    if (result.changes === 0) return this.findInOrg(id, organizationId);
    return this.findInOrg(id, organizationId);
  }
}
