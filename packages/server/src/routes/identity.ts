import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { cedarGate } from '../policy/gate.js';
import { mintApiKey } from '../identity/apikey.js';
import { mintSVID } from '../identity/svid.js';
import { applyMigrations } from '@promptsheon/shared';
import type Database from 'better-sqlite3';
import { randomUUID } from 'node:crypto';
import { NotFoundError } from '@promptsheon/shared';

const MintKeySchema = z.object({
  agentId: z.string().min(1).max(128),
  organizationId: z.string().min(1).max(128),
  ttlDays: z.number().int().min(1).max(365).optional().default(30),
  scope: z.string().max(2048).optional(),
});

const MintSvidSchema = z.object({
  agentId: z.string().min(1).max(128),
  organizationId: z.string().min(1).max(128),
  signingKeyPem: z.string().min(1), // PKCS8 PEM
  ttlSeconds: z.number().int().min(60).max(86_400).optional().default(900),
  scope: z.array(z.string()).optional(),
  classification: z.string().min(1).max(64).optional(),
});

interface AgentIdentityRow {
  id: string;
  agent_id: string;
  organization_id: string;
  mode: 'apikey' | 'svid';
  credential: string;
  scope: string;
  issued_at: string;
  expires_at: string;
  revoked_at: string | null;
}

function rowToObject(row: AgentIdentityRow): {
  id: string;
  agentId: string;
  organizationId: string;
  mode: 'apikey' | 'svid';
  scope: string;
  issuedAt: string;
  expiresAt: string;
  revokedAt: string | null;
} {
  return {
    id: row.id,
    agentId: row.agent_id,
    organizationId: row.organization_id,
    mode: row.mode,
    scope: row.scope,
    issuedAt: row.issued_at,
    expiresAt: row.expires_at,
    revokedAt: row.revoked_at,
  };
}

export interface IdentityDeps {
  db: Database.Database;
}

/**
 * Register the agent-identity routes. Three endpoints:
 *
 *  - POST /api/identity/keys   — mint a long-lived apikey
 *  - POST /api/identity/svid   — mint a short-lived SVID
 *  - DELETE /api/identity/:id  — revoke (sets revoked_at)
 *
 * All three are Cedar-gated. The Cedar policy in
 * `policies/promptsheon.cedar` covers them via:
 *   - Identity::Mint (apikey + SVID)
 *   - Identity::Revoke
 * The gates install the action name on the request and the
 * policy decides whether the principal can mint / revoke.
 */
export function registerIdentityRoutes(app: FastifyInstance, deps: IdentityDeps): void {
  // Ensure migrations ran (idempotent — applyMigrations no-ops if
  // the schema is already up to date).
  applyMigrations(deps.db, loadAllMigrations());

  const adminOnly = cedarGate({ action: 'Identity::Mint', resource: 'default' });
  const adminOrApproverRevoke = cedarGate({
    action: 'Identity::Revoke',
    resource: 'default',
  });

  app.post(
    '/api/identity/keys',
    { preHandler: adminOnly },
    async (request, reply) => {
      const parsed = MintKeySchema.safeParse(request.body);
      if (!parsed.success) {
        return reply.code(422).send({
          error: {
            code: 'VALIDATION_ERROR',
            message: 'invalid mint-key body',
            issues: parsed.error.issues,
          },
        });
      }
      const material = mintApiKey({
        agentId: parsed.data.agentId,
        orgId: parsed.data.organizationId,
        ttlDays: parsed.data.ttlDays,
        scope: parsed.data.scope,
      });
      const id = randomUUID();
      deps.db
        .prepare(
          `INSERT INTO agent_identities (id, agent_id, organization_id, mode, credential, scope, issued_at, expires_at)
           VALUES (?, ?, ?, 'apikey', ?, ?, ?, ?)`,
        )
        .run(
          id,
          material.agentId,
          material.orgId,
          material.hash,
          material.scope,
          material.issuedAt,
          material.expiresAt,
        );
      return reply.code(201).send({
        id,
        token: material.token,
        hash: material.hash,
        agentId: material.agentId,
        organizationId: material.orgId,
        scope: material.scope,
        issuedAt: material.issuedAt,
        expiresAt: material.expiresAt,
      });
    },
  );

  app.post(
    '/api/identity/svid',
    { preHandler: adminOnly },
    async (request, reply) => {
      const parsed = MintSvidSchema.safeParse(request.body);
      if (!parsed.success) {
        return reply.code(422).send({
          error: {
            code: 'VALIDATION_ERROR',
            message: 'invalid mint-svid body',
            issues: parsed.error.issues,
          },
        });
      }
      let token: string;
      try {
        token = mintSVID({
          agentId: parsed.data.agentId,
          orgId: parsed.data.organizationId,
          signingKey: parsed.data.signingKeyPem,
          ttlSeconds: parsed.data.ttlSeconds,
          scope: parsed.data.scope,
          classification: parsed.data.classification,
        });
      } catch (err) {
        return reply.code(400).send({
          error: { code: 'INVALID_SIGNING_KEY', message: (err as Error).message },
        });
      }
      // The SVID signature is the verification primitive; we
      // store the token (so the verifier can re-validate the
      // signature, and so the operator can audit what's active).
      // The credential column here is the *token* itself, not a
      // hash — for SVIDs, the verifier re-runs ed25519 verification
      // from the token + the operator's public key.
      const id = randomUUID();
      const issuedAt = new Date().toISOString();
      const expiresAt = new Date(Date.now() + (parsed.data.ttlSeconds ?? 900) * 1000).toISOString();
      deps.db
        .prepare(
          `INSERT INTO agent_identities (id, agent_id, organization_id, mode, credential, scope, issued_at, expires_at)
           VALUES (?, ?, ?, 'svid', ?, ?, ?, ?)`,
        )
        .run(id, parsed.data.agentId, parsed.data.organizationId, token, '[]', issuedAt, expiresAt);
      return reply.code(201).send({
        id,
        token,
        agentId: parsed.data.agentId,
        organizationId: parsed.data.organizationId,
        scope: parsed.data.scope ?? ['gateway', 'memory', 'tool'],
        classification: parsed.data.classification ?? 'internal',
        issuedAt,
        expiresAt,
      });
    },
  );

  app.delete(
    '/api/identity/:id',
    { preHandler: adminOrApproverRevoke },
    async (request, reply) => {
      const { id } = request.params as { id: string };
      const row = deps.db
        .prepare('SELECT * FROM agent_identities WHERE id = ?')
        .get(id) as AgentIdentityRow | undefined;
      if (!row) {
        return reply.code(404).send({
          error: { code: 'IDENTITY_NOT_FOUND', message: `identity ${id} not found` },
        });
      }
      if (row.mode === 'svid') {
        // SVIDs are short-lived; revocation is the more useful
        // semantic — insert into svid_revocations so the verifier
        // short-circuits to 401 even if the signature is valid.
        deps.db
          .prepare(
            `INSERT OR REPLACE INTO svid_revocations (svid_id, agent_id, revoked_at, reason)
             VALUES (?, ?, ?, 'manual revoke')`,
          )
          .run(row.credential, row.agent_id, new Date().toISOString());
      } else {
        // Apikeys are long-lived; mark revoked_at so the verifier
        // rejects them.
        deps.db
          .prepare('UPDATE agent_identities SET revoked_at = ? WHERE id = ?')
          .run(new Date().toISOString(), id);
      }
      return reply.code(204).send();
    },
  );
}

import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const MIGRATIONS_DIR = join(__dirname, '..', '..', '..', 'shared', 'db', 'migrations');

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

// Re-export so the public API is just the registration function.
export { rowToObject };
export type { AgentIdentityRow };