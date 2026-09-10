/**
 * Branch — a movable pointer (head commit oid) to a linear history
 * within a repository.
 */

export interface Branch {
  id: string;
  repositoryId: string;
  name: string;
  headCommitOid: string | null;
  isProtected: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface BranchCreateInput {
  repositoryId: string;
  name: string;
  headCommitOid?: string | null;
  isProtected?: boolean;
  fromBranch?: string;
}

export interface BranchUpdateInput {
  headCommitOid?: string;
  isProtected?: boolean;
}
