import type Database from 'better-sqlite3';
import { randomUUID } from 'node:crypto';
import type {
  MergeRequest,
  MergeRequestApproval,
  MergeRequestComment,
  MergeRequestCreateInput,
  MergeRequestDecisionInput,
} from '@promptsheon/shared';

interface MRRow {
  id: string;
  repository_id: string;
  number: number;
  title: string;
  description: string | null;
  source_branch: string;
  target_branch: string;
  source_commit_oid: string;
  merge_commit_oid: string | null;
  author_id: string;
  status: MergeRequest['status'];
  requested_reviewers: string;
  created_at: string;
  updated_at: string;
  merged_at: string | null;
}

interface ApprovalRow {
  merge_request_id: string;
  user_id: string;
  decision: 'approve' | 'request_changes';
  comment_id: string | null;
  created_at: string;
}

interface CommentRow {
  id: string;
  merge_request_id: string;
  author_id: string;
  path: string | null;
  body: string;
  created_at: string;
}

function toMR(row: MRRow): MergeRequest {
  const approvals = approvalsFor(row.id);
  return {
    id: row.id,
    repositoryId: row.repository_id,
    number: row.number,
    title: row.title,
    description: row.description,
    sourceBranch: row.source_branch,
    targetBranch: row.target_branch,
    sourceCommitOid: row.source_commit_oid,
    mergeCommitOid: row.merge_commit_oid,
    authorId: row.author_id,
    status: row.status,
    approvedBy: approvals.filter((a) => a.decision === 'approve').map((a) => a.userId),
    requestedReviewers: JSON.parse(row.requested_reviewers) as string[],
    createdAt: row.created_at,
    updatedAt: row.updated_at,
    mergedAt: row.merged_at,
  };
}

function approvalsFor(mrId: string): MergeRequestApproval[] {
  const db = (globalThis as unknown as { __mrRepoDb?: Database.Database }).__mrRepoDb;
  if (!db) return [];
  const rows = db
    .prepare('SELECT * FROM merge_request_approvals WHERE merge_request_id = ?')
    .all(mrId) as ApprovalRow[];
  return rows.map((r) => ({
    mergeRequestId: r.merge_request_id,
    userId: r.user_id,
    decision: r.decision,
    commentId: r.comment_id,
    createdAt: r.created_at,
  }));
}

export class MergeRequestRepo {
  constructor(private db: Database.Database) {
    (globalThis as unknown as { __mrRepoDb?: Database.Database }).__mrRepoDb = db;
  }

  listOpen(repositoryId: string): MergeRequest[] {
    const rows = this.db
      .prepare(
        `SELECT * FROM merge_requests WHERE repository_id = ? AND status = 'open'
         ORDER BY number DESC`,
      )
      .all(repositoryId) as MRRow[];
    return rows.map(toMR);
  }

  listAll(repositoryId: string): MergeRequest[] {
    const rows = this.db
      .prepare('SELECT * FROM merge_requests WHERE repository_id = ? ORDER BY number DESC')
      .all(repositoryId) as MRRow[];
    return rows.map(toMR);
  }

  findById(id: string): MergeRequest | null {
    const row = this.db
      .prepare('SELECT * FROM merge_requests WHERE id = ?')
      .get(id) as MRRow | undefined;
    return row ? toMR(row) : null;
  }

  nextNumber(repositoryId: string): number {
    const row = this.db
      .prepare('SELECT MAX(number) AS max FROM merge_requests WHERE repository_id = ?')
      .get(repositoryId) as { max: number | null };
    return (row?.max ?? 0) + 1;
  }

  create(input: MergeRequestCreateInput): MergeRequest {
    const id = randomUUID();
    const now = new Date().toISOString();
    const number = this.nextNumber(input.repositoryId);
    this.db
      .prepare(
        `INSERT INTO merge_requests (
            id, repository_id, number, title, description,
            source_branch, target_branch, source_commit_oid,
            author_id, requested_reviewers, created_at, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(
        id,
        input.repositoryId,
        number,
        input.title,
        input.description ?? null,
        input.sourceBranch,
        input.targetBranch,
        input.sourceCommitOid,
        input.authorId,
        JSON.stringify(input.requestedReviewers ?? []),
        now,
        now,
      );
    return this.findById(id)!;
  }

  setStatus(id: string, status: MergeRequest['status'], mergedCommitOid: string | null): MergeRequest | null {
    const now = new Date().toISOString();
    this.db
      .prepare(
        `UPDATE merge_requests SET status = ?, merge_commit_oid = ?,
           updated_at = ?, merged_at = CASE WHEN ? = 'merged' THEN ? ELSE merged_at END
         WHERE id = ?`,
      )
      .run(status, mergedCommitOid, now, status, now, id);
    return this.findById(id);
  }

  decide(mrId: string, input: MergeRequestDecisionInput, comment: MergeRequestComment | null): MergeRequestApproval {
    this.db
      .prepare(
        `INSERT INTO merge_request_approvals (
            merge_request_id, user_id, decision, comment_id, created_at
        ) VALUES (?, ?, ?, ?, ?)
        ON CONFLICT (merge_request_id, user_id) DO UPDATE SET
          decision = excluded.decision,
          comment_id = excluded.comment_id,
          created_at = excluded.created_at`,
      )
      .run(mrId, input.userId, input.decision, comment?.id ?? null, new Date().toISOString());
    return {
      mergeRequestId: mrId,
      userId: input.userId,
      decision: input.decision,
      commentId: comment?.id ?? null,
      createdAt: new Date().toISOString(),
    };
  }

  addComment(c: Omit<MergeRequestComment, 'id' | 'createdAt'>): MergeRequestComment {
    const id = randomUUID();
    const now = new Date().toISOString();
    this.db
      .prepare(
        `INSERT INTO merge_request_comments (id, merge_request_id, author_id, path, body, created_at)
         VALUES (?, ?, ?, ?, ?, ?)`,
      )
      .run(id, c.mergeRequestId, c.authorId, c.path, c.body, now);
    return { ...c, id, createdAt: now };
  }

  listComments(mrId: string): MergeRequestComment[] {
    const rows = this.db
      .prepare(
        'SELECT * FROM merge_request_comments WHERE merge_request_id = ? ORDER BY created_at ASC',
      )
      .all(mrId) as CommentRow[];
    return rows.map((r) => ({
      id: r.id,
      mergeRequestId: r.merge_request_id,
      authorId: r.author_id,
      path: r.path,
      body: r.body,
      createdAt: r.created_at,
    }));
  }

  listApprovals(mrId: string): MergeRequestApproval[] {
    return approvalsFor(mrId);
  }
}
