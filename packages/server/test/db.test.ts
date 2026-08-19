import { describe, it, expect, beforeEach } from 'vitest';
import Database from 'better-sqlite3';
import { runMigrations } from '../src/db/index.js';
import { WorkspaceRepo } from '../src/repos/workspace.js';

function createDb(): Database.Database {
  const db = new Database(':memory:');
  db.pragma('foreign_keys = ON');
  return db;
}

describe('db & WorkspaceRepo', () => {
  let db: Database.Database;
  let repo: WorkspaceRepo;

  beforeEach(async () => {
    db = createDb();
    await runMigrations(db);
    repo = new WorkspaceRepo(db);
  });

  it('runs all migrations', () => {
    const rows = db.prepare('SELECT version, name FROM _migrations ORDER BY version').all() as Array<{
      version: number;
      name: string;
    }>;
    expect(rows.length).toBe(23);
    expect(rows[0].name).toBe('001_core_schema.up.sql');
    expect(rows[rows.length - 1].name).toBe('024_releases_updated_at.up.sql');
  });

  it('creates a workspace and finds it by id', () => {
    const created = repo.create({ name: 'acme', organization: 'ACME Inc' });
    expect(created.id).toBeTypeOf('string');
    expect(created.name).toBe('acme');
    expect(created.organization).toBe('ACME Inc');

    const found = repo.findById(created.id);
    expect(found).not.toBeNull();
    expect(found?.id).toBe(created.id);
    expect(found?.name).toBe('acme');
  });

  it('updates a workspace', () => {
    const created = repo.create({ name: 'acme' });
    const updated = repo.update(created.id, { name: 'acme-renamed', organization: 'NewCo' });

    expect(updated.id).toBe(created.id);
    expect(updated.name).toBe('acme-renamed');
    expect(updated.organization).toBe('NewCo');
    expect(updated.updatedAt >= created.updatedAt).toBe(true);

    const refetched = repo.findById(created.id);
    expect(refetched?.name).toBe('acme-renamed');
    expect(refetched?.organization).toBe('NewCo');
  });

  it('deletes a workspace', () => {
    const created = repo.create({ name: 'doomed' });
    expect(repo.findById(created.id)).not.toBeNull();

    repo.delete(created.id);
    expect(repo.findById(created.id)).toBeFalsy();
  });
});
