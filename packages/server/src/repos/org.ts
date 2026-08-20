import type Database from 'better-sqlite3';
import { randomUUID } from 'node:crypto';
import type { Org, OrgMember, OrgRole, Team, TeamMember } from '@promptsheon/shared';
import { BaseRepo } from './base.js';

export class OrgRepo extends BaseRepo<Org> {
  constructor(db: Database.Database) {
    super(db, 'orgs');
  }

  findBySlug(slug: string): Org | null {
    return this.db.prepare('SELECT * FROM orgs WHERE slug = ?').get(slug) as Org | null;
  }

  create(data: { name: string; slug: string }): Org {
    const id = randomUUID();
    const now = new Date().toISOString();
    this.db.prepare(`INSERT INTO orgs (id, name, slug, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`)
      .run(id, data.name, data.slug, now, now);
    return { id, name: data.name, slug: data.slug, createdAt: now, updatedAt: now };
  }

  update(id: string, data: Partial<Pick<Org, 'name' | 'slug'>>): Org | null {
    const existing = this.findById(id);
    if (!existing) return null;
    const merged = { ...existing, ...data, updatedAt: new Date().toISOString() };
    this.db.prepare(`UPDATE orgs SET name = ?, slug = ?, updated_at = ? WHERE id = ?`)
      .run(merged.name, merged.slug, merged.updatedAt, id);
    return merged;
  }
}

export class TeamRepo extends BaseRepo<Team> {
  constructor(db: Database.Database) {
    super(db, 'teams');
  }

  findByOrgId(orgId: string): Team[] {
    return this.db.prepare('SELECT * FROM teams WHERE org_id = ?').all(orgId) as Team[];
  }

  create(data: { orgId: string; name: string }): Team {
    const id = randomUUID();
    const now = new Date().toISOString();
    this.db.prepare(`INSERT INTO teams (id, org_id, name, created_at) VALUES (?, ?, ?, ?)`)
      .run(id, data.orgId, data.name, now);
    return { id, orgId: data.orgId, name: data.name, createdAt: now };
  }
}

export class MembershipRepo {
  constructor(private db: Database.Database) {}

  addOrgMember(orgId: string, userId: string, role: OrgRole): OrgMember {
    const joinedAt = new Date().toISOString();
    this.db.prepare(`
      INSERT INTO org_members (org_id, user_id, role, joined_at)
      VALUES (?, ?, ?, ?)
      ON CONFLICT(org_id, user_id) DO UPDATE SET role = excluded.role
    `).run(orgId, userId, role, joinedAt);
    return { orgId, userId, role, joinedAt };
  }

  removeOrgMember(orgId: string, userId: string): boolean {
    const result = this.db.prepare('DELETE FROM org_members WHERE org_id = ? AND user_id = ?')
      .run(orgId, userId);
    return result.changes > 0;
  }

  findOrgMembers(orgId: string): OrgMember[] {
    const rows = this.db.prepare('SELECT * FROM org_members WHERE org_id = ?').all(orgId) as Array<{
      org_id: string;
      user_id: string;
      role: string;
      joined_at: string;
    }>;
    return rows.map((row) => ({
      orgId: row.org_id,
      userId: row.user_id,
      role: row.role as OrgRole,
      joinedAt: row.joined_at,
    }));
  }

  findOrgsForUser(userId: string): string[] {
    return (this.db.prepare('SELECT org_id FROM org_members WHERE user_id = ?')
      .all(userId) as Array<{ org_id: string }>).map((r) => r.org_id);
  }

  userHasOrgRole(userId: string, orgId: string, allowedRoles: OrgRole[]): boolean {
    const row = this.db.prepare('SELECT role FROM org_members WHERE org_id = ? AND user_id = ?')
      .get(orgId, userId) as { role: OrgRole } | undefined;
    if (!row) return false;
    return allowedRoles.includes(row.role);
  }

  addTeamMember(teamId: string, userId: string): TeamMember {
    const joinedAt = new Date().toISOString();
    this.db.prepare(`
      INSERT INTO team_members (team_id, user_id, joined_at)
      VALUES (?, ?, ?)
      ON CONFLICT(team_id, user_id) DO NOTHING
    `).run(teamId, userId, joinedAt);
    return { teamId, userId, joinedAt };
  }

  findTeamMembers(teamId: string): TeamMember[] {
    return this.db.prepare('SELECT * FROM team_members WHERE team_id = ?').all(teamId) as TeamMember[];
  }
}