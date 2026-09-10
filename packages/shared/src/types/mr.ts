/**
 * MergeRequest — a request to merge one branch into another within a
 * repository, gated on author ≠ approver, signature on the merged
 * commit, and a configurable minimum approver count.
 *
 * Inline comments live alongside in `merge_request_comments`.
 */

export type MergeRequestStatus = 'open' | 'merged' | 'closed';

export interface MergeRequest {
  id: string;
  repositoryId: string;
  number: number;
  title: string;
  description: string | null;
  sourceBranch: string;
  targetBranch: string;
  sourceCommitOid: string;
  mergeCommitOid: string | null;
  authorId: string;
  status: MergeRequestStatus;
  approvedBy: string[];
  requestedReviewers: string[];
  createdAt: string;
  updatedAt: string;
  mergedAt: string | null;
}

export interface MergeRequestApproval {
  mergeRequestId: string;
  userId: string;
  decision: 'approve' | 'request_changes';
  commentId: string | null;
  createdAt: string;
}

export interface MergeRequestComment {
  id: string;
  mergeRequestId: string;
  authorId: string;
  path: string | null;
  body: string;
  createdAt: string;
}

export interface MergeRequestCreateInput {
  repositoryId: string;
  title: string;
  description?: string | null;
  sourceBranch: string;
  targetBranch: string;
  sourceCommitOid: string;
  authorId: string;
  requestedReviewers?: string[];
}

export interface MergeRequestDecisionInput {
  userId: string;
  decision: 'approve' | 'request_changes';
  comment?: string;
}
