import { describe, expect, it } from 'vitest';
import Database from 'better-sqlite3';
import { applyMigrations } from '@promptsheon/shared';
import { DatasetRepo } from '../src/repos/dataset.js';

function openDb(): Database.Database {
  const db = new Database(':memory:');
  db.pragma('foreign_keys = ON');
  applyMigrations(db, [{
    version: 1,
    name: '001_core_schema.up.sql',
    up: `CREATE TABLE orgs (id TEXT PRIMARY KEY, name TEXT NOT NULL, slug TEXT NOT NULL UNIQUE, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
      CREATE TABLE workspaces (id TEXT PRIMARY KEY, name TEXT NOT NULL, organization TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
      CREATE TABLE projects (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL, name TEXT NOT NULL, description TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
      CREATE TABLE capabilities (id TEXT PRIMARY KEY, project_id TEXT NOT NULL, name TEXT NOT NULL, description TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
      CREATE TABLE datasets (id TEXT PRIMARY KEY, capability_id TEXT NOT NULL, name TEXT NOT NULL, description TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
      CREATE TABLE dataset_cases (id TEXT PRIMARY KEY, dataset_id TEXT NOT NULL, seq INTEGER NOT NULL, inputs TEXT NOT NULL, expected TEXT NOT NULL, description TEXT NOT NULL);`,
  }, { version: 25, name: '025_user_org_team.up.sql', up: 'ALTER TABLE workspaces ADD COLUMN org_id TEXT DEFAULT \'\';' }]);
  return db;
}

describe('DatasetRepo organization scoping', () => {
  it('does not expose or mutate datasets across organizations', () => {
    const db = openDb();
    db.prepare("INSERT INTO orgs VALUES ('org-a','A','a','now','now'), ('org-b','B','b','now','now')").run();
    db.prepare("INSERT INTO workspaces (id,name,organization,created_at,updated_at,org_id) VALUES ('ws-a','A','A','now','now','org-a'), ('ws-b','B','B','now','now','org-b')").run();
    db.prepare("INSERT INTO projects VALUES ('project-a','ws-a','A','', 'now','now'), ('project-b','ws-b','B','', 'now','now')").run();
    db.prepare("INSERT INTO capabilities VALUES ('cap-a','project-a','A','', 'now','now'), ('cap-b','project-b','B','', 'now','now')").run();
    db.prepare("INSERT INTO datasets VALUES ('dataset-a','cap-a','A','', 'now','now'), ('dataset-b','cap-b','B','', 'now','now')").run();
    db.prepare("INSERT INTO dataset_cases VALUES ('case-a','dataset-a',1,'{}','{}','')").run();

    const repo = new DatasetRepo(db);
    expect(repo.findByIdInOrg('dataset-a', 'org-a')?.id).toBe('dataset-a');
    expect(repo.findByIdInOrg('dataset-a', 'org-b')).toBeNull();
    expect(repo.findCasesInOrg('dataset-a', 'org-b')).toBeNull();
    expect(repo.deleteCaseInOrg('case-a', 'dataset-a', 'org-b')).toBe(false);
    expect(repo.deleteInOrg('dataset-a', 'org-b')).toBe(false);
    expect(repo.createInOrg({ capabilityId: 'cap-a', name: 'copy' }, 'org-b')).toBeNull();
    expect(db.prepare('SELECT id FROM datasets WHERE id = ?').get('dataset-a')).toBeTruthy();
    db.close();
  });
});
