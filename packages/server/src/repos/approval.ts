import type Database from 'better-sqlite3';
import type { Approval } from '@promptsheon/shared';

export class ApprovalRepo {
  constructor(private db: Database.Database) {}

  getByReleaseId(releaseId: string): Approval | null {
    return this.db.prepare('SELECT * FROM approvals WHERE release_id = ?').get(releaseId) as Approval | null;
  }

  listAll(): Approval[] {
    const rows = this.db
      .prepare('SELECT release_id, votes, updated_at FROM approvals ORDER BY updated_at DESC')
      .all() as Array<{ release_id: string; votes: string; updated_at: string }>;
    return rows.map((row) => ({ releaseId: row.release_id, votes: row.votes, updatedAt: row.updated_at }));
  }

  upsert(releaseId: string, votes: string): boolean {
    const existing = this.getByReleaseId(releaseId);
    const now = new Date().toISOString();
    if (existing) {
      this.db.prepare('UPDATE approvals SET votes = ?, updated_at = ? WHERE release_id = ?')
        .run(votes, now, releaseId);
    } else {
      this.db.prepare('INSERT INTO approvals (release_id, votes, updated_at) VALUES (?, ?, ?)')
        .run(releaseId, votes, now);
    }
    return true;
  }
}
