import type Database from 'better-sqlite3';

export interface SearchResult {
  kind: string;
  resourceId: string;
  title: string;
  body: string;
}

/** Provides full-text search over the materialized application index. */
export class SearchRepo {
  constructor(private readonly db: Database.Database) {}

  search(query: string, kind?: string): SearchResult[] {
    const where = kind ? 'AND kind = ?' : '';
    const params: unknown[] = [query];
    if (kind) params.push(kind);
    const rows = this.db
      .prepare(
        `SELECT kind, resource_id, title, body
         FROM search_index
         WHERE search_index MATCH ? ${where}
         ORDER BY rank
         LIMIT 50`,
      )
      .all(...params) as Array<{
      kind: string;
      resource_id: string;
      title: string;
      body: string;
    }>;
    return rows.map((row) => ({
      kind: row.kind,
      resourceId: row.resource_id,
      title: row.title,
      body: row.body,
    }));
  }
}
