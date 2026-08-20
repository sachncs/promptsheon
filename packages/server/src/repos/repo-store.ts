import type Database from 'better-sqlite3';
import { Buffer } from 'node:buffer';
import { createHash } from 'node:crypto';

export interface RepoEntry {
  path: string;
  blobOid: string;
  size: number;
}

/**
 * Content-addressed identifiers for the repo store. The blob oid is
 * `sha256(content)`; the tree oid is `sha256(canonicalJson(entries))`.
 */
export function sha256(content: Buffer | string): string {
  return createHash('sha256').update(content).digest('hex');
}

export function canonicalJson(input: Record<string, string>): string {
  const sorted = Object.keys(input)
    .sort()
    .reduce<Record<string, string>>((acc, k) => {
      acc[k] = input[k];
      return acc;
    }, {});
  return JSON.stringify(sorted);
}

export function treeOid(entries: Record<string, string>): string {
  return sha256(canonicalJson(entries));
}

interface BlobRow {
  oid: string;
  size: number;
  content: Buffer;
}

/**
 * RepoStore — file-backed content store scoped to (repository, ref).
 *
 * - putFile(repoId, ref, path, content) — stage a file
 * - getFile / readContent               — read the staged content
 * - list                                — enumerate the current tree
 * - pinTree                            — compute + pin the tree oid
 *
 * Trees are content-addressed: same set of (path, blob_oid) pairs
 * always yields the same tree oid.
 */
export class RepoStore {
  constructor(private db: Database.Database) {}

  putFile(repositoryId: string, ref: string, path: string, content: Buffer | string): RepoEntry {
    const buf = typeof content === 'string' ? Buffer.from(content, 'utf-8') : content;
    const oid = sha256(buf);
    this.db
      .prepare(
        `INSERT INTO repo_blobs (oid, size, content) VALUES (?, ?, ?)
         ON CONFLICT(oid) DO NOTHING`,
      )
      .run(oid, buf.byteLength, buf);

    this.db
      .prepare(
        `INSERT INTO repo_trees (repository_id, ref, path, blob_oid, updated_at)
         VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
         ON CONFLICT (repository_id, ref, path) DO UPDATE SET
           blob_oid = excluded.blob_oid,
           updated_at = CURRENT_TIMESTAMP`,
      )
      .run(repositoryId, ref, path, oid);

    return { path, blobOid: oid, size: buf.byteLength };
  }

  getFile(repositoryId: string, ref: string, path: string): RepoEntry | null {
    const row = this.db
      .prepare(
        `SELECT rt.blob_oid, rb.size FROM repo_trees rt
         LEFT JOIN repo_blobs rb ON rb.oid = rt.blob_oid
         WHERE rt.repository_id = ? AND rt.ref = ? AND rt.path = ?`,
      )
      .get(repositoryId, ref, path) as { blob_oid: string; size: number | null } | undefined;
    if (!row) return null;
    return { path, blobOid: row.blob_oid, size: row.size ?? 0 };
  }

  readContent(repositoryId: string, ref: string, path: string): Buffer | null {
    const meta = this.getFile(repositoryId, ref, path);
    if (!meta) return null;
    const row = this.db
      .prepare('SELECT content FROM repo_blobs WHERE oid = ?')
      .get(meta.blobOid) as BlobRow | undefined;
    return row ? row.content : null;
  }

  list(repositoryId: string, ref: string): RepoEntry[] {
    const rows = this.db
      .prepare(
        `SELECT rt.path, rt.blob_oid, rb.size
         FROM repo_trees rt
         LEFT JOIN repo_blobs rb ON rb.oid = rt.blob_oid
         WHERE rt.repository_id = ? AND rt.ref = ?
         ORDER BY rt.path ASC`,
      )
      .all(repositoryId, ref) as Array<{ path: string; blob_oid: string; size: number | null }>;
    return rows.map((r) => ({ path: r.path, blobOid: r.blob_oid, size: r.size ?? 0 }));
  }

  deleteFile(repositoryId: string, ref: string, path: string): boolean {
    const res = this.db
      .prepare('DELETE FROM repo_trees WHERE repository_id = ? AND ref = ? AND path = ?')
      .run(repositoryId, ref, path);
    return res.changes > 0;
  }

  pinTree(
    repositoryId: string,
    ref: string,
  ): { treeOid: string; entries: Record<string, string> } {
    const entries: Record<string, string> = {};
    for (const e of this.list(repositoryId, ref)) {
      entries[e.path] = e.blobOid;
    }
    const oid = treeOid(entries);
    canonicalJson(entries); // canonical form is also produced; pinning keeps the deterministic oid

    this.db
      .prepare(
        `INSERT INTO repo_pinned_trees (repository_id, ref, tree_oid, committed_at)
         VALUES (?, ?, ?, CURRENT_TIMESTAMP)
         ON CONFLICT (repository_id, ref) DO UPDATE SET
           tree_oid = excluded.tree_oid,
           committed_at = CURRENT_TIMESTAMP`,
      )
      .run(repositoryId, ref, oid);

    return { treeOid: oid, entries };
  }

  getPinnedTree(
    repositoryId: string,
    ref: string,
  ): { treeOid: string; entries: Record<string, string> } | null {
    const row = this.db
      .prepare('SELECT tree_oid FROM repo_pinned_trees WHERE repository_id = ? AND ref = ?')
      .get(repositoryId, ref) as { tree_oid: string } | undefined;
    if (!row) return null;
    const entries: Record<string, string> = {};
    for (const e of this.list(repositoryId, ref)) {
      entries[e.path] = e.blobOid;
    }
    return { treeOid: row.tree_oid, entries };
  }
}
