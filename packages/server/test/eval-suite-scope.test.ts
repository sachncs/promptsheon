import { describe, expect, it } from 'vitest';
import Database from 'better-sqlite3';
import { EvalSuiteRepo, HumanReviewRepo } from '../src/repos/eval-suite.js';

describe('eval suite organization scope', () => {
  it('keeps suites, versions, and human reviews inside the requested organization', () => {
    const db = new Database(':memory:');
    db.exec(`
      CREATE TABLE workspaces (id TEXT PRIMARY KEY, org_id TEXT NOT NULL);
      CREATE TABLE projects (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL);
      CREATE TABLE capabilities (id TEXT PRIMARY KEY, project_id TEXT NOT NULL);
      CREATE TABLE eval_suites (
        id TEXT PRIMARY KEY, capability_id TEXT NOT NULL, repository_id TEXT,
        name TEXT NOT NULL, description TEXT, current_version INTEGER NOT NULL,
        pass_threshold REAL NOT NULL, borderline_band REAL NOT NULL,
        created_by TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
      );
      CREATE TABLE eval_suite_versions (
        id TEXT PRIMARY KEY, suite_id TEXT NOT NULL, version INTEGER NOT NULL,
        grader_config TEXT NOT NULL, pass_threshold REAL NOT NULL,
        borderline_band REAL NOT NULL, k INTEGER NOT NULL, n INTEGER NOT NULL,
        notes TEXT, created_by TEXT NOT NULL, created_at TEXT NOT NULL
      );
      CREATE TABLE human_review_queue (
        id TEXT PRIMARY KEY, case_id TEXT NOT NULL, suite_id TEXT NOT NULL,
        suite_run_id TEXT, submitted_at TEXT NOT NULL, reviewer_id TEXT,
        decided_at TEXT, decision TEXT, notes TEXT
      );
    `);
    db.prepare('INSERT INTO workspaces VALUES (?, ?)').run('ws-a', 'org-a');
    db.prepare('INSERT INTO workspaces VALUES (?, ?)').run('ws-b', 'org-b');
    db.prepare('INSERT INTO projects VALUES (?, ?)').run('project-a', 'ws-a');
    db.prepare('INSERT INTO projects VALUES (?, ?)').run('project-b', 'ws-b');
    db.prepare('INSERT INTO capabilities VALUES (?, ?)').run('cap-a', 'project-a');
    db.prepare('INSERT INTO capabilities VALUES (?, ?)').run('cap-b', 'project-b');

    const suiteRow = (id: string, capabilityId: string) => [
      id, capabilityId, null, `suite-${id}`, null, 1, 0.9, 0.05, 'user', '2026-01-01', '2026-01-01',
    ];
    const insertSuite = db.prepare(
      `INSERT INTO eval_suites
       (id, capability_id, repository_id, name, description, current_version,
        pass_threshold, borderline_band, created_by, created_at, updated_at)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    );
    insertSuite.run(...suiteRow('suite-a', 'cap-a'));
    insertSuite.run(...suiteRow('suite-b', 'cap-b'));
    db.prepare(
      `INSERT INTO eval_suite_versions
       (id, suite_id, version, grader_config, pass_threshold, borderline_band,
        k, n, notes, created_by, created_at)
       VALUES ('version-a', 'suite-a', 1, '[]', 0.9, 0.05, 1, 1, NULL, 'user', '2026-01-01')`,
    ).run();
    db.prepare(
      `INSERT INTO human_review_queue
       (id, case_id, suite_id, suite_run_id, submitted_at)
       VALUES ('review-a', 'case-a', 'suite-a', NULL, '2026-01-01'),
              ('review-b', 'case-b', 'suite-b', NULL, '2026-01-01')`,
    ).run();

    const suites = new EvalSuiteRepo(db);
    const reviews = new HumanReviewRepo(db);
    expect(suites.listInOrg('org-a')).toHaveLength(1);
    expect(suites.findByIdInOrg('suite-b', 'org-a')).toBeNull();
    expect(suites.findVersionByIdInOrg('version-a', 'org-a')?.suiteId).toBe('suite-a');
    expect(suites.findVersionByIdInOrg('version-a', 'org-b')).toBeNull();
    expect(reviews.listOpenInOrg('org-a').map((review) => review.id)).toEqual(['review-a']);
    expect(reviews.decideInOrg('review-b', 'org-a', 'reviewer', 'approve', null)).toBeNull();
    expect(reviews.findById('review-b')?.decision).toBeNull();
    db.close();
  });
});
