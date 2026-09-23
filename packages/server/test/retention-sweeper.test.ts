import { describe, expect, it } from 'vitest';
import Database from 'better-sqlite3';
import { runMigrations } from '../src/db/index.js';
import { RetentionSweeper } from '../src/scheduler/retention-sweeper.js';

describe('RetentionSweeper', () => {
  it('uses each organization retention policy and keeps newer rows', async () => {
    const db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    await runMigrations(db);
    db.prepare(`INSERT INTO orgs (id, name, slug, created_at, updated_at) VALUES ('org-a', 'A', 'a', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).run();
    db.prepare(`INSERT INTO orgs (id, name, slug, created_at, updated_at) VALUES ('org-b', 'B', 'b', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).run();
    db.prepare(`INSERT INTO users (id, org_id, email, name, role, created_at, updated_at) VALUES ('user-a', 'org-a', 'a@example.com', 'A', 'admin', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).run();
    db.prepare(`INSERT INTO workspaces (id, org_id, name, organization, created_at, updated_at) VALUES ('ws-a', 'org-a', 'A', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).run();
    db.prepare(`INSERT INTO workspaces (id, org_id, name, organization, created_at, updated_at) VALUES ('ws-b', 'org-b', 'B', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).run();
    db.prepare(`INSERT INTO projects (id, workspace_id, name, description, created_at, updated_at) VALUES ('project-a', 'ws-a', 'A', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).run();
    db.prepare(`INSERT INTO projects (id, workspace_id, name, description, created_at, updated_at) VALUES ('project-b', 'ws-b', 'B', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).run();
    db.prepare(`INSERT INTO capabilities (id, project_id, name, description, created_at, updated_at) VALUES ('cap-a', 'project-a', 'A', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).run();
    db.prepare(`INSERT INTO capabilities (id, project_id, name, description, created_at, updated_at) VALUES ('cap-b', 'project-b', 'B', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).run();
    db.prepare(`INSERT INTO releases (id, capability_id, capability_version, manifest, environment, status, created_at, created_by) VALUES ('release-a', 'cap-a', 1, '{}', 'prod', 'active', CURRENT_TIMESTAMP, 'user-a')`).run();
    db.prepare(`INSERT INTO datasets (id, capability_id, name, description, created_at, updated_at) VALUES ('dataset-a', 'cap-a', 'A', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).run();
    db.prepare(`INSERT INTO eval_suites (id, capability_id, name, created_by, created_at, updated_at) VALUES ('suite-a', 'cap-a', 'A', 'user-a', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).run();
    db.prepare(`INSERT INTO system_config (key, value, updated_by) VALUES ('org.retention.days.org-a', '30', 'system')`).run();
    db.prepare(`INSERT INTO eval_runs (id, release_id, dataset_id, scorer, started_at) VALUES ('run-old', 'release-a', 'dataset-a', 'test', '2020-01-01T00:00:00.000Z')`).run();
    db.prepare(`INSERT INTO eval_results (id, run_id, seq, actual) VALUES ('result-old', 'run-old', 1, '{}')`).run();
    db.prepare(`INSERT INTO human_review_queue (id, case_id, suite_id, submitted_at) VALUES ('review-old', 'case-a', 'suite-a', '2020-01-01T00:00:00.000Z')`).run();

    const auditEntries: unknown[] = [];
    const sweeper = new RetentionSweeper(db, { append: (entry) => auditEntries.push(entry) }, () => new Date('2026-01-01T00:00:00.000Z'));
    const results = sweeper.sweepOnce('org-a');

    expect(results).toEqual([
      { table: 'eval_results', deletedRows: 1, cutoff: '2025-12-02T00:00:00.000Z' },
      { table: 'human_review_queue', deletedRows: 1, cutoff: '2025-12-02T00:00:00.000Z' },
    ]);
    expect(db.prepare('SELECT COUNT(*) AS count FROM eval_results').get()).toEqual({ count: 0 });
    expect(db.prepare('SELECT COUNT(*) AS count FROM human_review_queue').get()).toEqual({ count: 0 });
    expect(auditEntries).toHaveLength(1);
    db.close();
  });
});
