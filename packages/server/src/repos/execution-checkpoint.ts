import type Database from 'better-sqlite3';
import { z } from 'zod';

export interface ExecutionCheckpoint {
  executionId: string;
  stepId: string;
  state: 'completed' | 'failed';
  output: string;
  metadata: Record<string, unknown>;
  createdAt: string;
}

interface CheckpointRow {
  execution_id: string;
  step_id: string;
  state: 'completed' | 'failed';
  output: string;
  metadata: string;
  created_at: string;
}

/** Durable per-step checkpoint storage for recoverable executions. */
export class ExecutionCheckpointRepo {
  constructor(private readonly db: Database.Database) {}

  save(input: Omit<ExecutionCheckpoint, 'createdAt'>): ExecutionCheckpoint {
    const createdAt = new Date().toISOString();
    this.db.prepare(`
      INSERT INTO execution_checkpoints (execution_id, step_id, state, output, metadata, created_at)
      VALUES (?, ?, ?, ?, ?, ?)
      ON CONFLICT(execution_id, step_id) DO UPDATE SET
        state = excluded.state, output = excluded.output, metadata = excluded.metadata, created_at = excluded.created_at
    `).run(input.executionId, input.stepId, input.state, input.output, JSON.stringify(input.metadata), createdAt);
    return { ...input, createdAt };
  }

  list(executionId: string): ExecutionCheckpoint[] {
    const rows = this.db.prepare(
      'SELECT * FROM execution_checkpoints WHERE execution_id = ? ORDER BY created_at ASC, step_id ASC',
    ).all(executionId) as CheckpointRow[];
    return rows.map((row) => ({
      executionId: row.execution_id,
      stepId: row.step_id,
      state: row.state,
      output: row.output,
      metadata: z.record(z.string(), z.unknown()).parse(JSON.parse(row.metadata)),
      createdAt: row.created_at,
    }));
  }

  clear(executionId: string): number {
    return this.db.prepare('DELETE FROM execution_checkpoints WHERE execution_id = ?').run(executionId).changes;
  }
}
