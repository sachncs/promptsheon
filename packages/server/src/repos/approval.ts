import type Database from 'better-sqlite3';
import type { Approval } from '@promptsheon/shared';

export class ApprovalRepo {
  constructor(private db: Database.Database) {}

  getByReleaseId(releaseId: string): Approval | null {
    return this.db.prepare('SELECT * FROM approvals WHERE release_id = ?').get(releaseId) as Approval | null;
  }

  getByReleaseIdInOrg(releaseId: string, organizationId: string): Approval | null {
    const row = this.db
      .prepare(
        `SELECT a.*
         FROM approvals a
         JOIN releases r ON r.id = a.release_id
         JOIN capabilities c ON c.id = r.capability_id
         JOIN projects p ON p.id = c.project_id
         JOIN workspaces w ON w.id = p.workspace_id
         WHERE a.release_id = ? AND w.org_id = ?`,
      )
      .get(releaseId, organizationId) as { release_id: string; votes: string; updated_at: string } | undefined;
    return row ? { releaseId: row.release_id, votes: row.votes, updatedAt: row.updated_at } : null;
  }

  listAll(): Approval[] {
    const rows = this.db
      .prepare('SELECT release_id, votes, updated_at FROM approvals ORDER BY updated_at DESC')
      .all() as Array<{ release_id: string; votes: string; updated_at: string }>;
    return rows.map((row) => ({ releaseId: row.release_id, votes: row.votes, updatedAt: row.updated_at }));
  }

  listAllForOrg(organizationId: string): Approval[] {
    const rows = this.db
      .prepare(
        `SELECT a.release_id, a.votes, a.updated_at
         FROM approvals a
         JOIN releases r ON r.id = a.release_id
         JOIN capabilities c ON c.id = r.capability_id
         JOIN projects p ON p.id = c.project_id
         JOIN workspaces w ON w.id = p.workspace_id
         WHERE w.org_id = ?
         ORDER BY a.updated_at DESC`,
      )
      .all(organizationId) as Array<{ release_id: string; votes: string; updated_at: string }>;
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
