import type Database from 'better-sqlite3';
import { randomUUID } from 'node:crypto';
import { camelize } from './base.js';

export interface Team {
  id: string;
  organizationId: string;
  name: string;
  slug: string;
  description: string;
  createdAt: string;
  updatedAt: string;
}

export interface TeamMember {
  teamId: string;
  userId: string;
  role: 'owner' | 'admin' | 'member' | 'viewer';
  createdAt: string;
}

interface TeamRow extends Record<string, unknown> {
  id: string;
  organisation_id: string;
  name: string;
  slug: string;
  description: string;
  created_at: string;
  updated_at: string;
}

interface TeamMemberRow {
  team_id: string;
  user_id: string;
  role: string;
  created_at: string;
}

function rowToTeam(r: TeamRow): Team {
  // team row has 'organisation_id' which cardin camelize's regex
  // (_([a-z0-9]) → uppercase) converts to 'organisationId'.
  // That doesn't match the Team interface's 'organizationId'.
  // Map explicitly.
  return {
    id: r['id'] as string,
    organizationId: r['organisation_id'] as string,
    name: r['name'] as string,
    slug: r['slug'] as string,
    description: r['description'] as string,
    createdAt: r['created_at'] as string,
    updatedAt: r['updated_at'] as string,
  };
}

function rowToMember(r: TeamMemberRow & { joined_at?: string }): TeamMember {
  return {
    teamId: r.team_id,
    userId: r.user_id,
    role: r.role as TeamMember['role'],
    createdAt: r.joined_at ?? r.created_at,
  };
}

/**
 * TeamRepo — per-org teams + per-team RBAC. Migration 047
 * altered the teams table to add slug/description/updated_at
 * columns and renamed org_id → organisation_id. Members live in
 * team_members (already added back in migration 025).
 */
export class TeamRepo {
  constructor(private db: Database.Database) {}

  create(input: {
    organizationId: string;
    name: string;
    slug: string;
    description?: string;
  }): Team {
    const id = randomUUID();
    const now = new Date().toISOString();
    this.db
      .prepare(
        `INSERT INTO teams (id, org_id, organisation_id, name, slug, description, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
      )
      .run(id, input.organizationId, input.organizationId, input.name, input.slug, input.description ?? '', now, now);
    return this.findById(id)!;
  }

  findById(id: string): Team | null {
    const row = this.db.prepare('SELECT * FROM teams WHERE id = ?').get(id) as TeamRow | undefined;
    return row ? rowToTeam(row) : null;
  }

  listByOrg(organizationId: string): Team[] {
    const rows = this.db
      .prepare('SELECT * FROM teams WHERE organisation_id = ? ORDER BY name ASC')
      .all(organizationId) as TeamRow[];
    return rows.map(rowToTeam);
  }

  addMember(
    teamId: string,
    userId: string,
    role: TeamMember['role'] = 'member',
  ): TeamMember {
    const now = new Date().toISOString();
    this.db
      .prepare(
        `INSERT INTO team_members (team_id, user_id, role, created_at, joined_at)
         VALUES (?, ?, ?, ?, ?)
         ON CONFLICT(team_id, user_id) DO UPDATE SET role = excluded.role`,
      )
      .run(teamId, userId, role, now, now);
    return { teamId, userId, role, createdAt: now };
  }

  removeMember(teamId: string, userId: string): boolean {
    const r = this.db
      .prepare('DELETE FROM team_members WHERE team_id = ? AND user_id = ?')
      .run(teamId, userId);
    return r.changes > 0;
  }

  listMembers(teamId: string): TeamMember[] {
    const rows = this.db
      .prepare('SELECT * FROM team_members WHERE team_id = ? ORDER BY joined_at ASC')
      .all(teamId) as TeamMemberRow[];
    return rows.map(rowToMember);
  }

  listTeamsForUser(userId: string): Team[] {
    const rows = this.db
      .prepare(
        `SELECT t.* FROM teams t
         JOIN team_members m ON m.team_id = t.id
         WHERE m.user_id = ?
         ORDER BY t.name ASC`,
      )
      .all(userId) as TeamRow[];
    return rows.map(rowToTeam);
  }

  /**
   * Effective role for a user in an org: highest of the user's
   * org-level role (passed in) and their max team role.
   */
  effectiveRole(userId: string, organizationId: string, orgRole: string): string {
    const teams = this.listTeamsForUser(userId).filter((t) => t.organizationId === organizationId);
    if (teams.length === 0) return orgRole;
    const order = ['viewer', 'member', 'admin', 'owner'];
    let best = order.indexOf(orgRole);
    for (const t of teams) {
      const role = this.bestRole(t.id, userId, order);
      if (role > best) best = role;
    }
    return order[best] ?? orgRole;
  }

  private bestRole(teamId: string, userId: string, order: string[]): number {
    const members = this.listMembers(teamId).filter((m) => m.userId === userId);
    if (members.length === 0) return -1;
    let best = -1;
    for (const m of members) {
      const idx = order.indexOf(m.role);
      if (idx > best) best = idx;
    }
    return best;
  }
}

/**
 * SsoConfigRepo — per-org OIDC connection settings. Secrets are
 * stored encrypted via the Vault; we never log or return the raw
 * client_secret.
 */
export class SsoConfigRepo {
  constructor(private db: Database.Database) {}

  upsert(input: {
    organizationId: string;
    provider: string;
    issuer: string;
    clientId: string;
    clientSecretEncrypted: string;
    scopes?: string;
    audience?: string | null;
    groupsClaim?: string;
    emailClaim?: string;
    nameClaim?: string;
    enabled?: boolean;
  }): void {
    const now = new Date().toISOString();
    this.db
      .prepare(
        `INSERT INTO sso_configs (
            organization_id, provider, issuer, client_id, client_secret_encrypted,
            scopes, audience, groups_claim, email_claim, name_claim,
            enabled, created_at, updated_at
         )
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
         ON CONFLICT(organization_id) DO UPDATE SET
           provider = excluded.provider,
           issuer = excluded.issuer,
           client_id = excluded.client_id,
           client_secret_encrypted = excluded.client_secret_encrypted,
           scopes = excluded.scopes,
           audience = excluded.audience,
           groups_claim = excluded.groups_claim,
           email_claim = excluded.email_claim,
           name_claim = excluded.name_claim,
           enabled = excluded.enabled,
           updated_at = excluded.updated_at`,
      )
      .run(
        input.organizationId,
        input.provider,
        input.issuer,
        input.clientId,
        input.clientSecretEncrypted,
        input.scopes ?? 'openid profile email groups',
        input.audience ?? null,
        input.groupsClaim ?? 'groups',
        input.emailClaim ?? 'email',
        input.nameClaim ?? 'name',
        input.enabled === false ? 0 : 1,
        now,
        now,
      );
  }

  get(organizationId: string): {
    organizationId: string;
    provider: string;
    issuer: string;
    clientId: string;
    clientSecretEncrypted: string;
    scopes: string;
    audience: string | null;
    groupsClaim: string;
    emailClaim: string;
    nameClaim: string;
    enabled: number;
  } | null {
    const row = this.db
      .prepare(
        'SELECT * FROM sso_configs WHERE organization_id = ?',
      )
      .get(organizationId) as
      | {
          organization_id: string;
          provider: string;
          issuer: string;
          client_id: string;
          client_secret_encrypted: string;
          scopes: string;
          audience: string | null;
          groups_claim: string;
          email_claim: string;
          name_claim: string;
          enabled: number;
        }
      | undefined;
    if (!row) return null;
    return {
      organizationId: row.organization_id,
      provider: row.provider,
      issuer: row.issuer,
      clientId: row.client_id,
      clientSecretEncrypted: row.client_secret_encrypted,
      scopes: row.scopes,
      audience: row.audience,
      groupsClaim: row.groups_claim,
      emailClaim: row.email_claim,
      nameClaim: row.name_claim,
      enabled: row.enabled,
    };
  }
}
