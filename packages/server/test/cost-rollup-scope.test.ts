import { describe, expect, it } from 'vitest';
import Database from 'better-sqlite3';
import { runMigrations } from '../src/db/index.js';
import { CostRollupRepo } from '../src/repos/vault-extras.js';

describe('CostRollupRepo organization scope', () => {
  it('only accepts capabilities belonging to the requested organization', async () => {
    const db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    await runMigrations(db);
    db.prepare(`INSERT INTO orgs (id, name, slug, created_at, updated_at) VALUES ('org-a', 'A', 'a', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).run();
    db.prepare(`INSERT INTO orgs (id, name, slug, created_at, updated_at) VALUES ('org-b', 'B', 'b', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).run();
    db.prepare(`INSERT INTO workspaces (id, org_id, name, organization, created_at, updated_at) VALUES ('ws-a', 'org-a', 'A', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).run();
    db.prepare(`INSERT INTO projects (id, workspace_id, name, description, created_at, updated_at) VALUES ('project-a', 'ws-a', 'A', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).run();
    db.prepare(`INSERT INTO capabilities (id, project_id, name, description, created_at, updated_at) VALUES ('cap-a', 'project-a', 'A', '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`).run();

    const repo = new CostRollupRepo(db);
    expect(repo.capabilityBelongsToOrg('cap-a', 'org-a')).toBe(true);
    expect(repo.capabilityBelongsToOrg('cap-a', 'org-b')).toBe(false);
    db.close();
  });
});
