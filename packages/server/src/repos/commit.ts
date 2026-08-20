import type Database from 'better-sqlite3';
import { createHash } from 'node:crypto';
import {
  type RepoCommit,
  type RepoCommitInput,
  commitInputPayload,
} from '@promptsheon/shared';

interface CommitRow {
  oid: string;
  repository_id: string;
  ref: string;
  tree_oid: string;
  parents: string;
  author_id: string;
  message: string;
  timestamp: string;
  signature: string | null;
  signed_key_id: string | null;
  signed_at: string | null;
}

function toCommit(row: CommitRow): RepoCommit {
  return {
    oid: row.oid,
    repositoryId: row.repository_id,
    ref: row.ref,
    treeOid: row.tree_oid,
    parents: JSON.parse(row.parents) as string[],
    authorId: row.author_id,
    message: row.message,
    timestamp: row.timestamp,
    signature: row.signature,
    signedKeyId: row.signed_key_id,
    signedAt: row.signed_at,
  };
}

interface CommitInputShape {
  treeOid: string;
  parents: string[];
  authorId: string;
  message: string;
  timestamp: string;
}

export function deriveCommitOid(input: CommitInputShape): string {
  const payload = commitInputPayload(input);
  return createHash('sha256').update(payload).digest('hex');
}

export class CommitRepo {
  constructor(private db: Database.Database) {}

  listForRef(repositoryId: string, ref: string): RepoCommit[] {
    const rows = this.db
      .prepare(
        `SELECT * FROM repo_commits WHERE repository_id = ? AND ref = ?
         ORDER BY timestamp DESC`,
      )
      .all(repositoryId, ref) as CommitRow[];
    return rows.map(toCommit);
  }

  findByOid(oid: string): RepoCommit | null {
    const row = this.db
      .prepare('SELECT * FROM repo_commits WHERE oid = ?')
      .get(oid) as CommitRow | undefined;
    return row ? toCommit(row) : null;
  }

  create(input: RepoCommitInput): RepoCommit {
    const timestamp = new Date().toISOString();
    const oid = deriveCommitOid({
      treeOid: input.treeOid,
      parents: input.parents,
      authorId: input.authorId,
      message: input.message,
      timestamp,
    });
    this.db
      .prepare(
        `INSERT OR IGNORE INTO repo_commits (
            oid, repository_id, ref, tree_oid, parents,
            author_id, message, timestamp
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(
        oid,
        input.repositoryId,
        input.ref,
        input.treeOid,
        JSON.stringify(input.parents),
        input.authorId,
        input.message,
        timestamp,
      );
    return this.findByOid(oid)!;
  }

  attachSignature(oid: string, signature: string, keyId: string, signedAt: string): RepoCommit | null {
    this.db
      .prepare(
        `UPDATE repo_commits
         SET signature = ?, signed_key_id = ?, signed_at = ?
         WHERE oid = ?`,
      )
      .run(signature, keyId, signedAt, oid);
    return this.findByOid(oid);
  }
}
