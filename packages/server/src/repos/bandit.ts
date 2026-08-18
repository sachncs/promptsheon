import type Database from 'better-sqlite3';

export interface BanditArmCounter {
  armId: string;
  replicaId: string;
  successes: number;
  failures: number;
}

export class BanditRepo {
  constructor(private db: Database.Database) {}

  get(armId: string, replicaId: string): BanditArmCounter | null {
    return this.db.prepare('SELECT * FROM bandit_arm_counters WHERE arm_id = ? AND replica_id = ?')
      .get(armId, replicaId) as BanditArmCounter | null;
  }

  increment(armId: string, replicaId: string, success: boolean): void {
    const existing = this.get(armId, replicaId);
    if (existing) {
      const col = success ? 'successes' : 'failures';
      this.db.prepare(`UPDATE bandit_arm_counters SET ${col} = ${col} + 1, updated_at = CURRENT_TIMESTAMP WHERE arm_id = ? AND replica_id = ?`)
        .run(armId, replicaId);
    } else {
      this.db.prepare('INSERT INTO bandit_arm_counters (arm_id, replica_id, successes, failures) VALUES (?, ?, ?, ?)')
        .run(armId, replicaId, success ? 1 : 0, success ? 0 : 1);
    }
  }

  loadAggregated(): Array<{ armId: string; successes: number; failures: number }> {
    return this.db.prepare('SELECT arm_id as armId, SUM(successes) as successes, SUM(failures) as failures FROM bandit_arm_counters GROUP BY arm_id')
      .all() as Array<{ armId: string; successes: number; failures: number }>;
  }
}
