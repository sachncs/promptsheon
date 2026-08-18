import Database from 'better-sqlite3';
import type { AppConfig } from '@promptsheon/shared';

export function createConnection(config: AppConfig): Database.Database {
  const db = new Database(config.server.dbPath);
  db.pragma('journal_mode = WAL');
  db.pragma('foreign_keys = ON');
  db.pragma('busy_timeout = 5000');
  return db;
}
