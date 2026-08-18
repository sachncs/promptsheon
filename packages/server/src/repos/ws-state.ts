import type Database from 'better-sqlite3';

export class WsStateRepo {
  constructor(private db: Database.Database) {}

  getNextId(): number {
    const row = this.db.prepare('SELECT next_id FROM ws_state WHERE id = 0').get() as { next_id: number } | undefined;
    return row?.next_id ?? 0;
  }

  setNextId(id: number): void {
    const existing = this.db.prepare('SELECT id FROM ws_state WHERE id = 0').get();
    if (existing) {
      this.db.prepare('UPDATE ws_state SET next_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 0').run(id);
    } else {
      this.db.prepare('INSERT INTO ws_state (id, next_id) VALUES (0, ?)').run(id);
    }
  }

  incrementNextId(): number {
    const current = this.getNextId();
    const next = current + 1;
    this.setNextId(next);
    return next;
  }
}
