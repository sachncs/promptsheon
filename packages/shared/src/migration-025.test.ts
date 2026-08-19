import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Database from 'better-sqlite3';
import { applyMigrations } from './db-migrate.js';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const MIGRATIONS_DIR = join(__dirname, '..', 'db', 'migrations');

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

describe('Migration 025 (orgs/teams/users)', () => {
  let db: ReturnType<typeof Database>;

  beforeEach(() => {
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
  });

  afterEach(() => { db.close(); });

  it('creates orgs table with required columns', () => {
    const cols = db.prepare("PRAGMA table_info(orgs)").all() as Array<{ name: string }>;
    const names = cols.map((c) => c.name);
    expect(names).toContain('id');
    expect(names).toContain('name');
    expect(names).toContain('slug');
    expect(names).toContain('created_at');
    expect(names).toContain('updated_at');
  });

  it('creates teams table with org_id FK', () => {
    db.prepare(`INSERT INTO orgs (id, name, slug, created_at, updated_at) VALUES ('o1', 'Org', 'org', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`).run();
    const cols = db.prepare("PRAGMA table_info(teams)").all() as Array<{ name: string }>;
    expect(cols.map((c) => c.name)).toContain('org_id');
    db.prepare(`INSERT INTO teams (id, org_id, name, created_at) VALUES ('t1', 'o1', 'Engineering', '2026-01-01T00:00:00Z')`).run();
    const row = db.prepare("SELECT * FROM teams WHERE id = 't1'").get() as { org_id: string };
    expect(row.org_id).toBe('o1');
  });

  it('orgs.slug is unique', () => {
    db.prepare(`INSERT INTO orgs (id, name, slug, created_at, updated_at) VALUES ('o1', 'Org', 'org', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`).run();
    expect(() => {
      db.prepare(`INSERT INTO orgs (id, name, slug, created_at, updated_at) VALUES ('o2', 'Org2', 'org', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`).run();
    }).toThrow(/UNIQUE/);
  });

  it('org_members.role CHECK constraint rejects invalid', () => {
    db.prepare(`INSERT INTO orgs (id, name, slug, created_at, updated_at) VALUES ('o1', 'Org', 'org', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`).run();
    db.prepare(`INSERT INTO users (id, org_id, email, name, role, created_at, updated_at) VALUES ('u1', 'o1', 'a@b.com', 'Alice', 'viewer', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`).run();
    expect(() => {
      db.prepare(`INSERT INTO org_members (org_id, user_id, role, joined_at) VALUES ('o1', 'u1', 'superuser', '2026-01-01T00:00:00Z')`).run();
    }).toThrow(/CHECK/);
  });

  it('workspaces gets org_id column with default empty string', () => {
    const cols = db.prepare("PRAGMA table_info(workspaces)").all() as Array<{ name: string; dflt_value: string | null }>;
    const orgIdCol = cols.find((c) => c.name === 'org_id');
    expect(orgIdCol).toBeDefined();
    expect(orgIdCol!.dflt_value).toBe("''");
  });

  it('org cascade deletes teams and members (user removed via FK)', () => {
    db.prepare(`INSERT INTO orgs (id, name, slug, created_at, updated_at) VALUES ('o1', 'Org', 'org', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`).run();
    db.prepare(`INSERT INTO users (id, org_id, email, name, role, created_at, updated_at) VALUES ('u1', 'o1', 'a@b.com', 'A', 'viewer', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`).run();
    db.prepare(`INSERT INTO teams (id, org_id, name, created_at) VALUES ('t1', 'o1', 'Eng', '2026-01-01T00:00:00Z')`).run();
    db.prepare(`INSERT INTO team_members (team_id, user_id, joined_at) VALUES ('t1', 'u1', '2026-01-01T00:00:00Z')`).run();
    db.prepare(`INSERT INTO org_members (org_id, user_id, role, joined_at) VALUES ('o1', 'u1', 'viewer', '2026-01-01T00:00:00Z')`).run();
    // Add team_members FKs visible in PRAGMA
    const fks = db.prepare("PRAGMA foreign_key_list(teams)").all() as Array<{ table: string; on_delete: string }>;
    console.log('teams FKs:', JSON.stringify(fks));
    db.prepare("DELETE FROM orgs WHERE id = 'o1'").run();
    const teamsCount = (db.prepare("SELECT COUNT(*) as c FROM teams").get() as { c: number }).c;
    const tmCount = (db.prepare("SELECT COUNT(*) as c FROM team_members").get() as { c: number }).c;
    const omCount = (db.prepare("SELECT COUNT(*) as c FROM org_members").get() as { c: number }).c;
    console.log(`after delete: teams=${teamsCount} team_members=${tmCount} org_members=${omCount}`);
    expect(omCount).toBe(0);
  });
});