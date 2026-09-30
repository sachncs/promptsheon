import { describe, expect, it } from 'vitest';
import Database from 'better-sqlite3';
import { applyMigrations } from '@promptsheon/shared';
import { GoalEvolutionRepo } from '../src/repos/goal-evolution.js';
import { readdirSync, readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import type { MigrationSql } from '@promptsheon/shared';

const migrationsDir = join(dirname(fileURLToPath(import.meta.url)), '..', '..', 'shared', 'db', 'migrations');

function migrations(): MigrationSql[] {
  return readdirSync(migrationsDir)
    .filter((name) => name.endsWith('.up.sql'))
    .map((name) => ({ version: Number.parseInt(name.split('_')[0] ?? '0', 10), name, up: readFileSync(join(migrationsDir, name), 'utf8') }))
    .filter((migration) => migration.version > 0)
    .sort((left, right) => left.version - right.version);
}

describe('GoalEvolutionRepo', () => {
  it('survives agent reconstruction and returns durable summaries', () => {
    const db = new Database(':memory:');
    applyMigrations(db, migrations());
    const first = new GoalEvolutionRepo(db);
    first.upsert('manifest-1', {
      currentHash: 'candidate-1',
      bestHash: 'candidate-1',
      bestScore: 0.91,
      iteration: 3,
      history: [{ iteration: 3, score: 0.91, manifestHash: 'candidate-1', at: '2026-09-30T00:00:00.000Z' }],
      totalCost: 0,
      snapshots: [],
    });

    const reconstructed = new GoalEvolutionRepo(db);
    expect(reconstructed.findByManifestHash('manifest-1')).toMatchObject({
      state: { currentHash: 'candidate-1', bestScore: 0.91, iteration: 3 },
    });
    expect(reconstructed.listSummaries()).toEqual([{
      manifestHash: 'manifest-1',
      bestScore: 0.91,
      iterations: 3,
      lastUpdated: expect.any(String),
    }]);
    db.close();
  });
});
