import type Database from 'better-sqlite3';
import type { Approval } from '@promptsheon/shared';

export class ApprovalRepo {
  constructor(private db: Database.Database) {}

  getByReleaseId(releaseId: string): Approval | null {
    return this.db.prepare('SELECT * FROM approvals WHERE release_id = ?').get(releaseId) as Approval | null;
  }

  upsert(releaseId: string, votes: string): void {
    const existing = this.getByReleaseId(releaseId);
    const now = new Date().toISOString();
    if (existing) {
      this.db.prepare('UPDATE approvals SET votes = ?, updated_at = ? WHERE release_id = ?')
        .run(votes, now, releaseId);
    } else {
      this.db.prepare('INSERT INTO approvals (release_id, votes, updated_at) VALUES (?, ?, ?)')
        .run(releaseId, votes, now);
    }
  }
}
