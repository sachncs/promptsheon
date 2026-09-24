import type Database from 'better-sqlite3';

/** Persisted agent credential metadata. */
export interface AgentIdentityRecord {
  id: string;
  agentId: string;
  organizationId: string;
  mode: 'apikey' | 'svid';
  credential: string;
  scope: string;
  issuedAt: string;
  expiresAt: string;
  revokedAt: string | null;
}

/** SQLite repository for agent credentials and revocation state. */
export class AgentIdentityRepo {
  constructor(private readonly db: Database.Database) {}

  create(input: Omit<AgentIdentityRecord, 'revokedAt'>): void {
    this.db.prepare(
      `INSERT INTO agent_identities (id, agent_id, organization_id, mode, credential, scope, issued_at, expires_at)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
    ).run(input.id, input.agentId, input.organizationId, input.mode, input.credential, input.scope, input.issuedAt, input.expiresAt);
  }

  findByIdInOrg(id: string, organizationId: string): AgentIdentityRecord | null {
    const row = this.db.prepare(
      `SELECT id, agent_id AS agentId, organization_id AS organizationId, mode,
              credential, scope, issued_at AS issuedAt, expires_at AS expiresAt,
              revoked_at AS revokedAt
       FROM agent_identities WHERE id = ? AND organization_id = ?`,
    ).get(id, organizationId) as AgentIdentityRecord | undefined;
    return row ?? null;
  }

  revokeSvid(svid: string, agentId: string, revokedAt: string): void {
    this.db.prepare(
      `INSERT OR REPLACE INTO svid_revocations (svid_id, agent_id, revoked_at, reason)
       VALUES (?, ?, ?, 'manual revoke')`,
    ).run(svid, agentId, revokedAt);
  }

  revokeApiKey(id: string, organizationId: string, revokedAt: string): void {
    this.db.prepare('UPDATE agent_identities SET revoked_at = ? WHERE id = ? AND organization_id = ?').run(revokedAt, id, organizationId);
  }
}
