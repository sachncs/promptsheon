import type Database from 'better-sqlite3';

export interface Paginated<T> {
  items: T[];
  total: number;
}

export class BaseRepo<T extends { id: string }> {
  constructor(
    protected db: Database.Database,
    protected table: string,
  ) {}

  findById(id: string): T | null {
    return this.db.prepare(`SELECT * FROM ${this.table} WHERE id = ?`).get(id) as T | null;
  }

  findMany(opts: { page: number; pageSize: number }): Paginated<T> {
    const total = (this.db.prepare(`SELECT COUNT(*) as count FROM ${this.table}`).get() as { count: number }).count;
    const items = this.db.prepare(`SELECT * FROM ${this.table} LIMIT ? OFFSET ?`)
      .all(opts.pageSize, (opts.page - 1) * opts.pageSize) as T[];
    return { items, total };
  }

  delete(id: string): boolean {
    const existing = this.findById(id);
    if (!existing) return false;
    this.db.prepare(`DELETE FROM ${this.table} WHERE id = ?`).run(id);
    return true;
  }
}