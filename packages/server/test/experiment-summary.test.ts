import { describe, it, expect, beforeEach } from 'vitest';
import Database from 'better-sqlite3';
import { applyMigrations } from '@promptsheon/shared';
import { ExperimentRepo } from '../src/repos/experiment.js';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const MIGRATIONS_DIR = join(__dirname, '..', '..', 'shared', 'db', 'migrations');

function loadAllMigrations() {
  return readdirSync(MIGRATIONS_DIR)
    .filter((f) => f.endsWith('.up.sql'))
    .map((f) => {
      const version = parseInt(f.split('_')[0], 10);
      const up = readFileSync(join(MIGRATIONS_DIR, f), 'utf-8');
      return { version, name: f, up };
    })
    .filter((m) => m.version !== 0)
    .sort((a, b) => a.version - b.version);
}

function makeDb(): Database.Database {
  const db = new Database(':memory:');
  // Disable foreign-key checks: the test only exercises the
  // experiment repo's summarize() logic and doesn't need the full
  // workspace → project → capability → release FK chain to be valid.
  db.pragma('foreign_keys = OFF');
  applyMigrations(db, loadAllMigrations());
  db.prepare(
    `INSERT INTO releases (id, capability_id, capability_version, manifest, environment, status, approved_by, created_at, created_by, activated_at)
     VALUES ('rel-1', 'cap1', 1, '{}', 'dev', 'active', '[]', '2026-01-01T00:00:00Z', 'tester', '2026-01-01T00:00:00Z')`,
  ).run();
  return db;
}

describe('ExperimentRepo.summarize', () => {
  let db: Database.Database;
  let repo: ExperimentRepo;

  beforeEach(() => {
    db = makeDb();
    repo = new ExperimentRepo(db);
  });

  it('returns null when no variants exist', () => {
    expect(repo.summarize('rel-x')).toBeNull();
  });

  it('returns null when no variant has observations', () => {
    repo.createVariant({ releaseId: 'rel-1', label: 'control', config: '{}', weight: 0.5 });
    repo.createVariant({ releaseId: 'rel-1', label: 'treatment', config: '{}', weight: 0.5 });
    expect(repo.summarize('rel-1')).toBeNull();
  });

  it('flags a clear winner when one variant dominates', () => {
    const control = repo.createVariant({ releaseId: 'rel-1', label: 'control', config: '{}', weight: 0.5 });
    const treatment = repo.createVariant({ releaseId: 'rel-1', label: 'treatment', config: '{}', weight: 0.5 });
    for (let i = 0; i < 50; i += 1) {
      repo.recordAssignment({ experimentId: control.id, caseId: `c${i}`, variantId: control.id, outcome: 'pass' });
      repo.recordAssignment({ experimentId: treatment.id, caseId: `c${i}`, variantId: treatment.id, outcome: 'fail' });
      // balance: control=50 passes, treatment=0
    }
    const summary = repo.summarize('rel-1', { bayesSamples: 1000 });
    expect(summary).not.toBeNull();
    expect(summary!.winner).toBe('control');
    expect(summary!.anySignificant).toBe(true);
  });

  it('reports inconclusive when rates are close', () => {
    const control = repo.createVariant({ releaseId: 'rel-1', label: 'control', config: '{}', weight: 0.5 });
    const treatment = repo.createVariant({ releaseId: 'rel-1', label: 'treatment', config: '{}', weight: 0.5 });
    for (let i = 0; i < 100; i += 1) {
      const outcome = i % 2 === 0 ? 'pass' : 'fail';
      repo.recordAssignment({ experimentId: control.id, caseId: `c${i}`, variantId: control.id, outcome });
      repo.recordAssignment({ experimentId: treatment.id, caseId: `c${i}`, variantId: treatment.id, outcome });
    }
    const summary = repo.summarize('rel-1', { bayesSamples: 500 });
    expect(summary).not.toBeNull();
    expect(summary!.anySignificant).toBe(false);
    expect(summary!.winner).toBeNull();
  });

  it('passes the alpha through to the pairwise tests', () => {
    const a = repo.createVariant({ releaseId: 'rel-1', label: 'a', config: '{}', weight: 0.5 });
    const b = repo.createVariant({ releaseId: 'rel-1', label: 'b', config: '{}', weight: 0.5 });
    for (let i = 0; i < 100; i += 1) {
      repo.recordAssignment({ experimentId: a.id, caseId: `c${i}`, variantId: a.id, outcome: i < 51 ? 'pass' : 'fail' });
      repo.recordAssignment({ experimentId: b.id, caseId: `c${i}`, variantId: b.id, outcome: i < 49 ? 'pass' : 'fail' });
    }
    // α = 1.0 (everything significant); α = 0 (nothing significant).
    expect(repo.summarize('rel-1', { alpha: 1.0, bayesSamples: 200 })!.anySignificant).toBe(true);
    expect(repo.summarize('rel-1', { alpha: 0.0, bayesSamples: 200 })!.anySignificant).toBe(false);
  });
});