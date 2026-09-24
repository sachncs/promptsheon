import { randomUUID } from 'node:crypto';
import { mintApiKey } from '../identity/apikey.js';
import { mintSVID } from '../identity/svid.js';
import type { AgentIdentityRecord } from '../repos/agent-identity.js';

/** Persistence boundary for agent credential lifecycle operations. */
export interface AgentIdentityStore {
  create(input: Omit<AgentIdentityRecord, 'revokedAt'>): void;
  findByIdInOrg(id: string, organizationId: string): AgentIdentityRecord | null;
  revokeSvid(svid: string, agentId: string, revokedAt: string): void;
  revokeApiKey(id: string, organizationId: string, revokedAt: string): void;
}

/** Application service for minting and revoking agent credentials. */
export class IdentityService {
  constructor(private readonly store: AgentIdentityStore) {}

  mintApiKey(input: { agentId: string; organizationId: string; ttlDays: number; scope?: string }) {
    const material = mintApiKey({
      agentId: input.agentId,
      orgId: input.organizationId,
      ttlDays: input.ttlDays,
      scope: input.scope,
    });
    const id = randomUUID();
    this.store.create({ id, agentId: material.agentId, organizationId: material.orgId, mode: 'apikey', credential: material.hash, scope: material.scope, issuedAt: material.issuedAt, expiresAt: material.expiresAt });
    return { id, ...material };
  }

  mintSvid(input: { agentId: string; organizationId: string; signingKeyPem: string; ttlSeconds: number; scope?: string[]; classification?: string }) {
    const token = mintSVID({ agentId: input.agentId, orgId: input.organizationId, signingKey: input.signingKeyPem, ttlSeconds: input.ttlSeconds, scope: input.scope, classification: input.classification });
    const issuedAt = new Date().toISOString();
    const expiresAt = new Date(Date.now() + input.ttlSeconds * 1000).toISOString();
    const id = randomUUID();
    const scope = input.scope ?? ['gateway', 'memory', 'tool'];
    this.store.create({ id, agentId: input.agentId, organizationId: input.organizationId, mode: 'svid', credential: token, scope: JSON.stringify(scope), issuedAt, expiresAt });
    return { id, token, agentId: input.agentId, organizationId: input.organizationId, scope, classification: input.classification ?? 'internal', issuedAt, expiresAt };
  }

  revoke(id: string, organizationId: string): boolean {
    const identity = this.store.findByIdInOrg(id, organizationId);
    if (!identity) return false;
    const revokedAt = new Date().toISOString();
    if (identity.mode === 'svid') this.store.revokeSvid(identity.credential, identity.agentId, revokedAt);
    else this.store.revokeApiKey(id, organizationId, revokedAt);
    return true;
  }
}
