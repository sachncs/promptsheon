import { describe, expect, it } from 'vitest';
import Database from 'better-sqlite3';
import { EvalRepo } from '../src/repos/eval.js';

describe('EvalRepo organization scope', () => {
  it('returns only runs belonging to the requested organization', () => {
    const db = new Database(':memory:');
    db.exec(`
      CREATE TABLE workspaces (id TEXT PRIMARY KEY, org_id TEXT NOT NULL);
      CREATE TABLE projects (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL);
      CREATE TABLE capabilities (id TEXT PRIMARY KEY, project_id TEXT NOT NULL);
      CREATE TABLE releases (id TEXT PRIMARY KEY, capability_id TEXT NOT NULL);
      CREATE TABLE eval_runs (id TEXT PRIMARY KEY, release_id TEXT, dataset_id TEXT, scorer TEXT, score REAL, passed INTEGER, failed INTEGER, total INTEGER, status TEXT, started_at TEXT, finished_at TEXT);
      CREATE TABLE eval_results (id TEXT PRIMARY KEY, run_id TEXT, case_id TEXT, seq INTEGER, passed INTEGER, actual TEXT, error TEXT, latency_ms INTEGER);
      CREATE TABLE datasets (id TEXT PRIMARY KEY, capability_id TEXT NOT NULL);
    `);
    db.prepare('INSERT INTO workspaces VALUES (?, ?)').run('ws-a', 'org-a');
    db.prepare('INSERT INTO workspaces VALUES (?, ?)').run('ws-b', 'org-b');
    db.prepare('INSERT INTO projects VALUES (?, ?)').run('project-a', 'ws-a');
    db.prepare('INSERT INTO projects VALUES (?, ?)').run('project-b', 'ws-b');
    db.prepare('INSERT INTO capabilities VALUES (?, ?)').run('cap-a', 'project-a');
    db.prepare('INSERT INTO capabilities VALUES (?, ?)').run('cap-b', 'project-b');
    db.prepare('INSERT INTO releases VALUES (?, ?)').run('release-a', 'cap-a');
    db.prepare('INSERT INTO releases VALUES (?, ?)').run('release-b', 'cap-b');
    db.prepare('INSERT INTO eval_runs VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)').run('run-a', 'release-a', 'dataset-a', 'test', 0.9, 1, 0, 1, 'passed', '2025-01-01', null);
    db.prepare('INSERT INTO eval_runs VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)').run('run-b', 'release-b', 'dataset-b', 'test', 0.1, 0, 1, 1, 'failed', '2025-01-01', null);

    const repo = new EvalRepo(db);
    const result = repo.findManyInOrg('org-a', { page: 1, pageSize: 25 });
    expect(result.total).toBe(1);
    expect(result.items[0]).toMatchObject({ id: 'run-a', releaseId: 'release-a', startedAt: '2025-01-01' });
    expect(repo.findRunByIdInOrg('run-b', 'org-a')).toBeNull();
    db.close();
  });
});
