import type Database from 'better-sqlite3';
import { randomUUID } from 'node:crypto';
import type { Tag, TagCreateInput } from '@promptsheon/shared';

interface TagRow {
  id: string;
  repository_id: string;
  name: string;
  commit_oid: string;
  message: string | null;
  tagger_id: string;
  created_at: string;
}

function toTag(row: TagRow): Tag {
  return {
    id: row.id,
    repositoryId: row.repository_id,
    name: row.name,
    commitOid: row.commit_oid,
    message: row.message,
    taggerId: row.tagger_id,
    createdAt: row.created_at,
  };
}

export class TagRepo {
  constructor(private db: Database.Database) {}

  list(repositoryId: string): Tag[] {
    const rows = this.db
      .prepare('SELECT * FROM tags WHERE repository_id = ? ORDER BY created_at DESC')
      .all(repositoryId) as TagRow[];
    return rows.map(toTag);
  }

  findByName(repositoryId: string, name: string): Tag | null {
    const row = this.db
      .prepare('SELECT * FROM tags WHERE repository_id = ? AND name = ?')
      .get(repositoryId, name) as TagRow | undefined;
    return row ? toTag(row) : null;
  }

  findByCommit(repositoryId: string, commitOid: string): Tag | null {
    const row = this.db
      .prepare('SELECT * FROM tags WHERE repository_id = ? AND commit_oid = ? LIMIT 1')
      .get(repositoryId, commitOid) as TagRow | undefined;
    return row ? toTag(row) : null;
  }

  create(input: TagCreateInput): Tag {
    const id = randomUUID();
    const now = new Date().toISOString();
    this.db
      .prepare(
        `INSERT INTO tags (id, repository_id, name, commit_oid, message, tagger_id, created_at)
         VALUES (?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(
        id,
        input.repositoryId,
        input.name,
        input.commitOid,
        input.message ?? null,
        input.taggerId,
        now,
      );
    return this.findByName(input.repositoryId, input.name)!;
  }

  delete(repositoryId: string, name: string): boolean {
    const res = this.db
      .prepare('DELETE FROM tags WHERE repository_id = ? AND name = ?')
      .run(repositoryId, name);
    return res.changes > 0;
  }
}
