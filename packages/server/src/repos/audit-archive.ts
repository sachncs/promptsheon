import type Database from 'better-sqlite3';

export class AuditArchiveRepo {
  constructor(private db: Database.Database) {}

  archive(olderThan: string): number {
    const result = this.db.prepare(`
      INSERT INTO audit_archive (id, user_id, action, resource, details, timestamp, previous_hash, entry_hash, timestamp_str, resource_kind, resource_id, archived_at)
      SELECT id, user_id, action, resource, details, timestamp, previous_hash, entry_hash, timestamp_str, resource_kind, resource_id, CURRENT_TIMESTAMP
      FROM audit_entries WHERE timestamp < ?
    `).run(olderThan);
    return result.changes;
  }

  search(opts: { page: number; pageSize: number; userId?: string; resourceKind?: string }): { items: any[]; total: number } {
    const conditions: string[] = [];
    const params: unknown[] = [];
    if (opts.userId) { conditions.push('user_id = ?'); params.push(opts.userId); }
    if (opts.resourceKind) { conditions.push('resource_kind = ?'); params.push(opts.resourceKind); }
    const where = conditions.length > 0 ? `WHERE ${conditions.join(' AND ')}` : '';
    const total = (this.db.prepare(`SELECT COUNT(*) as count FROM audit_archive ${where}`).get(...params) as { count: number }).count;
    const items = this.db.prepare(`SELECT * FROM audit_archive ${where} ORDER BY archived_at DESC LIMIT ? OFFSET ?`)
      .all(...params, opts.pageSize, (opts.page - 1) * opts.pageSize);
    return { items, total };
  }
}
