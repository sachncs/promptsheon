import type Database from 'better-sqlite3';
import type { EnforcerState } from '@promptsheon/shared';

export class EnforcerRepo {
  constructor(private db: Database.Database) {}

  get(workspaceId: string, kind: 'budget' | 'quota'): EnforcerState | null {
    return this.db.prepare('SELECT * FROM enforcer_state WHERE workspace_id = ? AND kind = ?')
      .get(workspaceId, kind) as EnforcerState | null;
  }

  upsert(state: EnforcerState): void {
    const existing = this.get(state.workspaceId, state.kind);
    if (existing) {
      this.db.prepare('UPDATE enforcer_state SET payload = ?, updated_at = CURRENT_TIMESTAMP WHERE workspace_id = ? AND kind = ?')
        .run(state.payload, state.workspaceId, state.kind);
    } else {
      this.db.prepare('INSERT INTO enforcer_state (workspace_id, kind, payload) VALUES (?, ?, ?)')
        .run(state.workspaceId, state.kind, state.payload);
    }
  }

  delete(workspaceId: string, kind: 'budget' | 'quota'): void {
    this.db.prepare('DELETE FROM enforcer_state WHERE workspace_id = ? AND kind = ?')
      .run(workspaceId, kind);
  }
}
