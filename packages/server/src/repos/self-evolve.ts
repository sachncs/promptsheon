import type Database from 'better-sqlite3';
import type { SelfEvolveState } from '@promptsheon/shared';

export class SelfEvolveRepo {
  constructor(private db: Database.Database) {}

  getByCapabilityAndEnv(capabilityId: string, targetEnv: string): SelfEvolveState | null {
    return this.db.prepare('SELECT * FROM self_evolve_state WHERE capability_id = ? AND target_env = ?')
      .get(capabilityId, targetEnv) as SelfEvolveState | null;
  }

  upsert(state: SelfEvolveState): void {
    const existing = this.getByCapabilityAndEnv(state.capabilityId, state.targetEnv);
    if (existing) {
      this.db.prepare(`
        UPDATE self_evolve_state SET last_attempt_at = ?, last_promote_at = ?, last_score = ?,
          last_revision_index = ?, cycle_started_at = ?, last_status = ?, last_error = ?
        WHERE capability_id = ? AND target_env = ?
      `).run(state.lastAttemptAt, state.lastPromoteAt, state.lastScore, state.lastRevisionIndex, state.cycleStartedAt, state.lastStatus, state.lastError, state.capabilityId, state.targetEnv);
    } else {
      this.db.prepare(`
        INSERT INTO self_evolve_state (capability_id, target_env, last_attempt_at, last_promote_at, last_score, last_revision_index, cycle_started_at, last_status, last_error)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
      `).run(state.capabilityId, state.targetEnv, state.lastAttemptAt, state.lastPromoteAt, state.lastScore, state.lastRevisionIndex, state.cycleStartedAt, state.lastStatus, state.lastError);
    }
  }

  findStale(maxAge: string): SelfEvolveState[] {
    return this.db.prepare("SELECT * FROM self_evolve_state WHERE last_status != 'idle' AND last_attempt_at < ?")
      .all(maxAge) as SelfEvolveState[];
  }
}
