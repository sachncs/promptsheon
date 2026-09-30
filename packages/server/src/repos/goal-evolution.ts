import type Database from 'better-sqlite3';
import type { Manifest } from '@promptsheon/shared';

/** A durable snapshot of one evolution iteration. */
export interface GoalEvolutionSnapshotRecord {
  iteration: number;
  manifestHash: string;
  manifest: Manifest;
  score: number;
  timestamp: string;
}

/** Durable state needed to resume goal-evolution observability after restart. */
export interface GoalEvolutionStateRecord {
  currentHash: string;
  bestHash: string;
  bestScore: number;
  iteration: number;
  history: Array<{ iteration: number; score: number; manifestHash: string; at: string }>;
  bestManifest?: Manifest;
  totalCost: number;
  snapshots: GoalEvolutionSnapshotRecord[];
}

/** Summary returned by the goal observability list endpoint. */
export interface GoalEvolutionSummaryRecord {
  manifestHash: string;
  bestScore: number;
  iterations: number;
  lastUpdated: string;
}

/** Persists goal-evolution state without coupling the agent to SQLite details. */
export class GoalEvolutionRepo {
  constructor(private readonly db: Database.Database) {}

  upsert(manifestHash: string, state: GoalEvolutionStateRecord): void {
    this.db.prepare(`
      INSERT INTO goal_evolution_state (manifest_hash, state_json, updated_at)
      VALUES (?, ?, ?)
      ON CONFLICT(manifest_hash) DO UPDATE SET
        state_json = excluded.state_json,
        updated_at = excluded.updated_at
    `).run(manifestHash, JSON.stringify(state), new Date().toISOString());
  }

  findByManifestHash(manifestHash: string): { state: GoalEvolutionStateRecord; updatedAt: string } | null {
    const row = this.db.prepare(`
      SELECT state_json, updated_at
      FROM goal_evolution_state
      WHERE manifest_hash = ?
    `).get(manifestHash) as { state_json: string; updated_at: string } | undefined;
    if (!row) return null;
    const parsed: unknown = JSON.parse(row.state_json);
    if (!isGoalEvolutionState(parsed)) throw new Error(`invalid persisted goal-evolution state for ${manifestHash}`);
    return { state: parsed, updatedAt: row.updated_at };
  }

  listSummaries(): GoalEvolutionSummaryRecord[] {
    const rows = this.db.prepare(`
      SELECT manifest_hash, state_json, updated_at
      FROM goal_evolution_state
      ORDER BY updated_at DESC
    `).all() as Array<{ manifest_hash: string; state_json: string; updated_at: string }>;
    return rows.map((row) => {
      const parsed: unknown = JSON.parse(row.state_json);
      if (!isGoalEvolutionState(parsed)) throw new Error(`invalid persisted goal-evolution state for ${row.manifest_hash}`);
      return { manifestHash: row.manifest_hash, bestScore: parsed.bestScore, iterations: parsed.iteration, lastUpdated: row.updated_at };
    });
  }
}

function isGoalEvolutionState(value: unknown): value is GoalEvolutionStateRecord {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return false;
  const candidate = value as Record<string, unknown>;
  return typeof candidate.currentHash === 'string'
    && typeof candidate.bestHash === 'string'
    && typeof candidate.bestScore === 'number'
    && Number.isFinite(candidate.bestScore)
    && Number.isInteger(candidate.iteration)
    && Array.isArray(candidate.history)
    && typeof candidate.totalCost === 'number'
    && Number.isFinite(candidate.totalCost)
    && Array.isArray(candidate.snapshots);
}
