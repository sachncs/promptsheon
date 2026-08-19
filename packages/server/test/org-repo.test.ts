import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Database from 'better-sqlite3';
import { OrgRepo, TeamRepo, MembershipRepo } from '../src/repos/org.js';
import { applyMigrations } from '@promptsheon/shared';
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

function insertTestData(db: ReturnType<typeof Database>): void {
  db.prepare(`INSERT INTO workspaces (id, name, organization, created_at, updated_at)
    VALUES ('ws1', 'Test', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`).run();
  db.prepare(`INSERT INTO projects (id, workspace_id, name, description, created_at, updated_at)
    VALUES ('proj1', 'ws1', 'P', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`).run();
  db.prepare(`INSERT INTO capabilities (id, project_id, name, description, created_at, updated_at)
    VALUES ('cap1', 'proj1', 'C', '', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`).run();
}

describe('OrgRepo', () => {
  let db: ReturnType<typeof Database>;
  let repo: OrgRepo;
  beforeEach(() => {
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
    insertTestData(db);
    repo = new OrgRepo(db);
  });
  afterEach(() => { db.close(); });

  it('creates and finds an org by id and slug', () => {
    const org = repo.create({ name: 'Acme', slug: 'acme' });
    expect(org.id).toBeTruthy();
    expect(repo.findById(org.id)?.name).toBe('Acme');
    expect(repo.findBySlug('acme')?.id).toBe(org.id);
  });

  it('rejects duplicate slug', () => {
    repo.create({ name: 'A', slug: 'same' });
    expect(() => repo.create({ name: 'B', slug: 'same' })).toThrow(/UNIQUE/);
  });

  it('updates an org', () => {
    const org = repo.create({ name: 'A', slug: 'a' });
    const updated = repo.update(org.id, { name: 'B' });
    expect(updated?.name).toBe('B');
  });
});

describe('TeamRepo', () => {
  let db: ReturnType<typeof Database>;
  let teamRepo: TeamRepo;
  let orgId: string;
  beforeEach(() => {
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
    insertTestData(db);
    teamRepo = new TeamRepo(db);
    const orgRepo = new OrgRepo(db);
    orgId = orgRepo.create({ name: 'O', slug: 'o' }).id;
  });
  afterEach(() => { db.close(); });

  it('creates a team in an org', () => {
    const team = teamRepo.create({ orgId, name: 'Engineering' });
    expect(team.orgId).toBe(orgId);
    expect(teamRepo.findByOrgId(orgId)).toHaveLength(1);
  });
});

describe('MembershipRepo', () => {
  let db: ReturnType<typeof Database>;
  let membershipRepo: MembershipRepo;
  let orgId: string;
  let teamId: string;
  beforeEach(() => {
    db = new Database(':memory:');
    db.pragma('foreign_keys = ON');
    applyMigrations(db, loadAllMigrations());
    insertTestData(db);
    membershipRepo = new MembershipRepo(db);
    const orgRepo = new OrgRepo(db);
    orgId = orgRepo.create({ name: 'O', slug: 'o' }).id;
    teamId = new TeamRepo(db).create({ orgId, name: 'Eng' }).id;
    db.prepare(`INSERT INTO users (id, org_id, email, name, role, created_at, updated_at)
      VALUES ('u1', '${orgId}', 'a@b.com', 'Alice', 'viewer', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`).run();
  });
  afterEach(() => { db.close(); });

  it('adds and finds an org member', () => {
    const m = membershipRepo.addOrgMember(orgId, 'u1', 'admin');
    expect(m.role).toBe('admin');
    expect(membershipRepo.findOrgMembers(orgId)).toHaveLength(1);
  });

  it('overwrites role on duplicate addOrgMember', () => {
    membershipRepo.addOrgMember(orgId, 'u1', 'viewer');
    membershipRepo.addOrgMember(orgId, 'u1', 'admin');
    expect(membershipRepo.findOrgMembers(orgId)[0]?.role).toBe('admin');
  });

  it('removes an org member', () => {
    membershipRepo.addOrgMember(orgId, 'u1', 'admin');
    expect(membershipRepo.removeOrgMember(orgId, 'u1')).toBe(true);
    expect(membershipRepo.findOrgMembers(orgId)).toHaveLength(0);
  });

  it('userHasOrgRole returns true for matching role', () => {
    membershipRepo.addOrgMember(orgId, 'u1', 'approver');
    expect(membershipRepo.userHasOrgRole('u1', orgId, ['admin', 'approver'])).toBe(true);
    expect(membershipRepo.userHasOrgRole('u1', orgId, ['admin'])).toBe(false);
  });

  it('userHasOrgRole returns false for non-member', () => {
    expect(membershipRepo.userHasOrgRole('nonexistent', orgId, ['admin'])).toBe(false);
  });

  it('findOrgsForUser returns all orgs', () => {
    const org2 = new OrgRepo(db).create({ name: 'O2', slug: 'o2' }).id;
    membershipRepo.addOrgMember(orgId, 'u1', 'viewer');
    membershipRepo.addOrgMember(org2, 'u1', 'viewer');
    const orgs = membershipRepo.findOrgsForUser('u1');
    expect(orgs.sort()).toEqual([org2, orgId].sort());
  });

  it('addTeamMember and findTeamMembers', () => {
    membershipRepo.addTeamMember(teamId, 'u1');
    expect(membershipRepo.findTeamMembers(teamId)).toHaveLength(1);
    expect(membershipRepo.addTeamMember(teamId, 'u1')).toBeDefined();  // no-op on conflict
  });
});