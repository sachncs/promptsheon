import type Database from 'better-sqlite3';
import { randomUUID } from 'node:crypto';
import type { Branch, BranchCreateInput, BranchUpdateInput } from '@promptsheon/shared';

interface BranchRow {
  id: string;
  repository_id: string;
  name: string;
  head_commit_oid: string | null;
  is_protected: number;
  created_at: string;
  updated_at: string;
}

function toBranch(row: BranchRow): Branch {
  return {
    id: row.id,
    repositoryId: row.repository_id,
    name: row.name,
    headCommitOid: row.head_commit_oid,
    isProtected: row.is_protected === 1,
    createdAt: row.created_at,
    updatedAt: row.updated_at,
  };
}

export class BranchRepo {
  constructor(private db: Database.Database) {}

  list(repositoryId: string): Branch[] {
    const rows = this.db
      .prepare('SELECT * FROM branches WHERE repository_id = ? ORDER BY name ASC')
      .all(repositoryId) as BranchRow[];
    return rows.map(toBranch);
  }

  findByName(repositoryId: string, name: string): Branch | null {
    const row = this.db
      .prepare('SELECT * FROM branches WHERE repository_id = ? AND name = ?')
      .get(repositoryId, name) as BranchRow | undefined;
    return row ? toBranch(row) : null;
  }

  create(input: BranchCreateInput): Branch {
    const id = randomUUID();
    const now = new Date().toISOString();
    const head = input.headCommitOid ?? null;
    this.db
      .prepare(
        `INSERT INTO branches (id, repository_id, name, head_commit_oid, is_protected, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(
        id,
        input.repositoryId,
        input.name,
        head,
        input.isProtected ? 1 : 0,
        now,
        now,
      );
    return this.findByName(input.repositoryId, input.name)!;
  }

  setHead(repositoryId: string, name: string, commitOid: string): Branch | null {
    const now = new Date().toISOString();
    this.db
      .prepare(
        `UPDATE branches SET head_commit_oid = ?, updated_at = ? WHERE repository_id = ? AND name = ?`,
      )
      .run(commitOid, now, repositoryId, name);
    return this.findByName(repositoryId, name);
  }

  delete(repositoryId: string, name: string): boolean {
    const res = this.db
      .prepare('DELETE FROM branches WHERE repository_id = ? AND name = ?')
      .run(repositoryId, name);
    return res.changes > 0;
  }

  update(repositoryId: string, name: string, patch: BranchUpdateInput): Branch | null {
    const existing = this.findByName(repositoryId, name);
    if (!existing) return null;
    const next = {
      ...existing,
      ...(patch.headCommitOid !== undefined ? { headCommitOid: patch.headCommitOid } : {}),
      ...(patch.isProtected !== undefined ? { isProtected: patch.isProtected } : {}),
      updatedAt: new Date().toISOString(),
    };
    this.db
      .prepare(
        `UPDATE branches SET head_commit_oid = ?, is_protected = ?, updated_at = ? WHERE repository_id = ? AND name = ?`,
      )
      .run(
        next.headCommitOid,
        next.isProtected ? 1 : 0,
        next.updatedAt,
        repositoryId,
        name,
      );
    return this.findByName(repositoryId, name);
  }
}
