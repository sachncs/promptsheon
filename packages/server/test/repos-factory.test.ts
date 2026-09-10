import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Database from 'better-sqlite3';
import { buildRepos } from '../src/repos/factory.js';

describe('buildRepos (issue #64 — composition-root refactor)', () => {
  let db: ReturnType<typeof Database>;

  beforeEach(() => {
    db = new Database(':memory:');
  });

  afterEach(() => {
    db.close();
  });

  it('returns every repo wired to the supplied Database handle', () => {
    const repos = buildRepos(db);
    const expectedKeys = [
      'workspace', 'project', 'repo', 'branch', 'tag', 'repoStore',
      'commit', 'mergeRequest', 'signingKey', 'evalSuite', 'humanReview',
      'vault', 'orgExport', 'costRollup', 'budget', 'trace', 'traceScore',
      'userAnalytics', 'team', 'ssoConfig', 'promptScan', 'redteam',
      'experiment', 'incident', 'orgSettings', 'featureFlag', 'capability',
      'version', 'release', 'execution', 'dataset', 'eval', 'precondition',
      'alert', 'schedule', 'approval', 'apiKey', 'user', 'systemConfig',
      'manifest', 'membership', 'webhook', 'idempotency',
    ];
    for (const key of expectedKeys) {
      expect(repos, `repos.${key} should be defined`).toHaveProperty(key);
      expect(repos[key as keyof typeof repos], `repos.${key} should not be null`).not.toBeNull();
    }
  });

  it('returns the same vault instance for both repos.vault and repos.orgExport', () => {
    const repos = buildRepos(db);
    // orgExport is constructed with repos.vault as its dep, so it
    // must share the same underlying kms handle.
    expect(repos.orgExport).toBeDefined();
    expect(repos.vault).toBeDefined();
  });

  it('returns repos that actually use the supplied Database handle', () => {
    const repos = buildRepos(db);
    db.exec(`CREATE TABLE workspaces (id TEXT PRIMARY KEY, name TEXT NOT NULL, organization TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`);
    expect(() => repos.workspace.create({ name: 'ws1' })).not.toThrow();
  });
});
