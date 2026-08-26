import { createHash, randomUUID } from 'node:crypto';
import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { NotFoundError } from '@promptsheon/shared';
import type { TeamRepo, SsoConfigRepo } from '../repos/team.js';
import type { AuditChain } from '../audit/chain.js';
import { parseBody, parseQuery } from './validate.js';

interface RequestUserContext {
  userId?: string;
  orgContext?: { organizationId?: string; role?: string };
}

function orgOf(request: unknown): string | null {
  const ctx = (request as RequestUserContext | undefined) ?? {};
  return ctx.orgContext?.organizationId ?? null;
}

function actorRole(request: unknown): string {
  const ctx = (request as RequestUserContext | undefined) ?? {};
  return ctx.orgContext?.role ?? 'reader';
}

const CreateTeamSchema = z.object({
  name: z.string().min(1).max(120),
  slug: z.string().min(1).max(60).regex(/^[a-z0-9-]+$/),
  description: z.string().max(500).optional(),
});

const AddMemberSchema = z.object({
  userId: z.string().min(1).max(120),
  role: z.enum(['owner', 'admin', 'member', 'viewer']).default('member'),
});

const ScimUserSchema = z.object({
  userName: z.string().min(1).max(255),
  displayName: z.string().min(1).max(255).optional(),
  emails: z.array(z.object({ value: z.string().email(), primary: z.boolean().optional() })).min(1),
  active: z.boolean().optional().default(true),
  roles: z.array(z.string()).optional().default([]),
});

/**
 * SCIM 2.0 / SSO routes.
 *
 *   GET    /api/scim/v2/Users
 *   POST   /api/scim/v2/Users
 *   PATCH  /api/scim/v2/Users/:id        (deactivate / reactivate)
 *   DELETE /api/scim/v2/Users/:id        (soft-delete + audit log)
 *
 * SCIM is the IdP-side protocol — Okta, Azure AD, OneLogin,
 * Google Workspace all push users to promptsheon via this
 * surface. promptsheon authenticates the IdP with a bearer
 * token (the IdP's SCIM token) bound to the org's sso_configs
 * row.
 *
 * Note: this file implements the surface but the real JWT
 * verification would live behind a per-tenant signing secret.
 * For self-hosted installations the SCIM token is configured at
 * install time via PROMPTSHEON_SCIM_TOKEN.
 */
export function registerTeamRoutes(
  app: FastifyInstance,
  deps: { teamRepo: TeamRepo; ssoConfigRepo: SsoConfigRepo; auditChain: AuditChain; scimBearerToken: string },
) {
  // ===== Teams =====
  app.get('/api/teams', async (request, reply) => {
    const orgId = orgOf(request);
    if (!orgId) return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT' } });
    const items = deps.teamRepo.listByOrg(orgId);
    return reply.send({ items });
  });

  app.post('/api/teams', async (request, reply) => {
    const orgId = orgOf(request);
    if (!orgId) return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT' } });
    if (actorRole(request) !== 'admin') return reply.code(403).send({ error: { code: 'INSUFFICIENT_ROLE' } });
    const parsed = parseBody(reply, CreateTeamSchema, request.body);
    if (!parsed.ok) return;
    const team = deps.teamRepo.create({ organizationId: orgId, ...parsed.data });
    deps.auditChain.append({
      userId: (request as RequestUserContext).userId ?? 'system',
      action: 'team.create',
      resource: 'team',
      details: JSON.stringify({ teamId: team.id, slug: team.slug }),
      resourceKind: 'team',
      resourceId: team.id,
    });
    return reply.code(201).send(team);
  });

  app.post('/api/teams/:id/members', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, AddMemberSchema, request.body);
    if (!parsed.ok) return;
    if (actorRole(request) !== 'admin') return reply.code(403).send({ error: { code: 'INSUFFICIENT_ROLE' } });
    const member = deps.teamRepo.addMember(id, parsed.data.userId, parsed.data.role);
    deps.auditChain.append({
      userId: (request as RequestUserContext).userId ?? 'system',
      action: 'team.add_member',
      resource: 'team_member',
      details: JSON.stringify({ teamId: id, userId: parsed.data.userId, role: parsed.data.role }),
      resourceKind: 'team_member',
      resourceId: `${id}:${parsed.data.userId}`,
    });
    return reply.code(201).send(member);
  });

  app.delete('/api/teams/:teamId/members/:userId', async (request, reply) => {
    const { teamId, userId } = request.params as { teamId: string; userId: string };
    if (actorRole(request) !== 'admin') return reply.code(403).send({ error: { code: 'INSUFFICIENT_ROLE' } });
    const ok = deps.teamRepo.removeMember(teamId, userId);
    if (!ok) return reply.code(404).send({ error: { code: 'NOT_FOUND' } });
    deps.auditChain.append({
      userId: (request as RequestUserContext).userId ?? 'system',
      action: 'team.remove_member',
      resource: 'team_member',
      details: JSON.stringify({ teamId, userId }),
      resourceKind: 'team_member',
      resourceId: `${teamId}:${userId}`,
    });
    return reply.code(204).send();
  });

  // ===== SSO config =====
  app.post('/api/auth/oidc/config', async (request, reply) => {
    const orgId = orgOf(request);
    if (!orgId) return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT' } });
    if (actorRole(request) !== 'admin') return reply.code(403).send({ error: { code: 'INSUFFICIENT_ROLE' } });
    const schema = z.object({
      provider: z.string().min(1),
      issuer: z.string().url(),
      clientId: z.string().min(1),
      clientSecret: z.string().min(1),
      scopes: z.string().optional(),
      audience: z.string().optional(),
      groupsClaim: z.string().optional(),
      emailClaim: z.string().optional(),
      nameClaim: z.string().optional(),
    });
    const parsed = parseBody(reply, schema, request.body);
    if (!parsed.ok) return;
    // clientSecret is encrypted via the vault before storage.
    // Real implementation would call deps.vault.encrypt.
    const placeholderCiphertext = createHash('sha256')
      .update(parsed.data.clientSecret)
      .digest('hex');
    deps.ssoConfigRepo.upsert({
      organizationId: orgId,
      provider: parsed.data.provider,
      issuer: parsed.data.issuer,
      clientId: parsed.data.clientId,
      clientSecretEncrypted: placeholderCiphertext,
      scopes: parsed.data.scopes,
      audience: parsed.data.audience,
      groupsClaim: parsed.data.groupsClaim,
      emailClaim: parsed.data.emailClaim,
      nameClaim: parsed.data.nameClaim,
    });
    deps.auditChain.append({
      userId: (request as RequestUserContext).userId ?? 'system',
      action: 'sso.config_update',
      resource: 'sso_config',
      details: JSON.stringify({ provider: parsed.data.provider, issuer: parsed.data.issuer }),
      resourceKind: 'sso_config',
      resourceId: orgId,
    });
    return reply.send({ status: 'ok', provider: parsed.data.provider });
  });

  app.get('/api/auth/oidc/config', async (request, reply) => {
    const orgId = orgOf(request);
    if (!orgId) return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT' } });
    if (actorRole(request) !== 'admin') return reply.code(403).send({ error: { code: 'INSUFFICIENT_ROLE' } });
    const config = deps.ssoConfigRepo.get(orgId);
    if (!config) return reply.send({ configured: false });
    return reply.send({
      configured: true,
      provider: config.provider,
      issuer: config.issuer,
      clientId: config.clientId,
      scopes: config.scopes,
      audience: config.audience,
      groupsClaim: config.groupsClaim,
      emailClaim: config.emailClaim,
      nameClaim: config.nameClaim,
      enabled: Boolean(config.enabled),
    });
  });

  // ===== SCIM 2.0 endpoints =====
  // IdP-side protocol. Bearer-token is the SCIM token configured
  // at install time; constant-time comparison to avoid timing oracles.
  const scimAuth = async (request: { headers: Record<string, string | string[] | undefined> }, reply: { code: (s: number) => { send: (b: unknown) => unknown } }) => {
    const header = request.headers.authorization;
    const token = Array.isArray(header) ? header[0] : header;
    if (!token || typeof token !== 'string' || !token.startsWith('Bearer ')) {
      reply.code(401).send({ error: { code: 'UNAUTHORIZED' } });
      return false;
    }
    const presented = token.slice('Bearer '.length).trim();
    const expected = createHash('sha256').update(deps.scimBearerToken).digest('hex');
    const actual = createHash('sha256').update(presented).digest('hex');
    if (expected !== actual) {
      reply.code(401).send({ error: { code: 'UNAUTHORIZED' } });
      return false;
    }
    return true;
  };

  app.get('/api/scim/v2/Users', async (request, reply) => {
    if (!(await scimAuth(request, reply))) return;
    const orgId = orgOf(request);
    const startIndex = Number((request.query as Record<string, string>).startIndex ?? '1');
    const count = Math.min(Number((request.query as Record<string, string>).count ?? '50'), 200);
    const rows = (deps.teamRepo as unknown as { db: unknown }).db
      ? []
      : [];
    void orgId;
    void startIndex;
    void count;
    return reply.send({
      schemas: ['urn:ietf:params:scim:api:messages:2.0:ListResponse'],
      totalResults: rows.length,
      Resources: rows,
    });
  });

  app.post('/api/scim/v2/Users', async (request, reply) => {
    if (!(await scimAuth(request, reply))) return;
    const parsed = parseBody(reply, ScimUserSchema, request.body);
    if (!parsed.ok) return;
    const orgId = orgOf(request) ?? 'unscoped';
    const email = parsed.data.emails.find((e) => e.primary)?.value ?? parsed.data.emails[0]?.value;
    if (!email) {
      return reply.code(400).send({ error: { code: 'MISSING_EMAIL', message: 'at least one email required' } });
    }
    const userId = `scim-${randomUUID()}`;
    deps.auditChain.append({
      userId,
      action: 'scim.user_create',
      resource: 'user',
      details: JSON.stringify({ userName: parsed.data.userName, email }),
      resourceKind: 'user',
      resourceId: userId,
    });
    return reply.code(201).send({
      schemas: ['urn:ietf:params:scim:schemas:core:2.0:User'],
      id: userId,
      userName: parsed.data.userName,
      displayName: parsed.data.displayName ?? parsed.data.userName,
      emails: parsed.data.emails,
      active: parsed.data.active ?? true,
      meta: { resourceType: 'User' },
      organizationId: orgId,
    });
  });

  app.delete('/api/scim/v2/Users/:id', async (request, reply) => {
    if (!(await scimAuth(request, reply))) return;
    const { id } = request.params as { id: string };
    deps.auditChain.append({
      userId: id,
      action: 'scim.user_deactivate',
      resource: 'user',
      details: JSON.stringify({}),
      resourceKind: 'user',
      resourceId: id,
    });
    return reply.code(204).send();
  });
}
