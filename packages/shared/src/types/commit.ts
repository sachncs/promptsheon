/**
 * Commit — a content-addressed reference to a tree plus author
 * metadata and zero or more parents. A commit oid is
 *
 *   sha256(tree_oid | parents[0..n] | authorId | timestamp | message)
 *
 * stable across the same logical commit. A signature blob is
 * optional (filled by the operator-managed key registry).
 */

export interface RepoCommit {
  oid: string;
  repositoryId: string;
  ref: string;
  treeOid: string;
  parents: string[];
  authorId: string;
  message: string;
  timestamp: string;
  signature?: string | null;
  signedKeyId?: string | null;
  signedAt?: string | null;
}

export interface RepoCommitInput {
  repositoryId: string;
  ref: string;
  treeOid: string;
  parents: string[];
  authorId: string;
  message: string;
}

export interface SignedCommitPayload {
  commitOid: string;
  ref: string;
  approverId: string;
  timestamp: string;
}

/**
 * Re-derive the canonical oid for a commit. Same inputs produce the
 * same oid across processes; signing does not affect the oid.
 */
export function commitInputPayload(input: {
  treeOid: string;
  parents: string[];
  authorId: string;
  timestamp: string;
  message: string;
}): string {
  return JSON.stringify({
    treeOid: input.treeOid,
    parents: [...input.parents].sort(),
    authorId: input.authorId,
    timestamp: input.timestamp,
    message: input.message,
  });
}
