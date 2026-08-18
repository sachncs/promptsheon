import type Database from 'better-sqlite3';

export function inTransaction<T>(db: Database.Database, fn: () => T): T {
  return db.transaction(fn)();
}
