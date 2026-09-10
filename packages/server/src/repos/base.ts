import type Database from 'better-sqlite3';

export interface Paginated<T> {
  items: T[];
  total: number;
}

/**
 * Convert a snake_case row from SQLite into a camelCase object.
 * Every repo in this project uses camelCase TypeScript interfaces,
 * but better-sqlite3 returns the raw column names. Without this
 * mapper, callers of `findById` got rows where `timeoutSec` was
 * undefined and `createdAt` was undefined (they live as
 * `timeout_sec` and `created_at` in the row). Several `update`
 * methods silently sent undefined into NOT NULL columns, which
 * was the latent source of bugs the previous audit flagged as
 * R5/R6.
 */
export function camelize<T extends Record<string, unknown>>(row: T): T {
  if (!row || typeof row !== 'object') return row;
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(row)) {
    const camel = k.includes('_')
      ? k.replace(/_([a-z0-9])/g, (_, c) => c.toUpperCase())
      : k;
    out[camel] = v;
  }
  return out as T;
}

export class BaseRepo<T extends { id: string }> {
  constructor(
    protected db: Database.Database,
    protected table: string,
  ) {}

  findById(id: string): T | null {
    const row = this.db.prepare(`SELECT * FROM ${this.table} WHERE id = ?`).get(id) as
      | Record<string, unknown>
      | undefined;
    if (!row) return null;
    return camelize<Record<string, unknown>>(row) as unknown as T;
  }

  findMany(opts: { page: number; pageSize: number }): Paginated<T> {
    const total = (this.db.prepare(`SELECT COUNT(*) as count FROM ${this.table}`).get() as { count: number }).count;
    const rows = this.db.prepare(`SELECT * FROM ${this.table} LIMIT ? OFFSET ?`)
      .all(opts.pageSize, (opts.page - 1) * opts.pageSize) as Array<Record<string, unknown>>;
    return { items: rows.map((r) => camelize<Record<string, unknown>>(r)) as unknown as T[], total };
  }

  delete(id: string): boolean {
    const existing = this.findById(id);
    if (!existing) return false;
    this.db.prepare(`DELETE FROM ${this.table} WHERE id = ?`).run(id);
    return true;
  }
}