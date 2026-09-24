import { createHash, randomUUID, timingSafeEqual } from 'node:crypto';
import type { FastifyInstance, FastifyRequest } from 'fastify';
import { z } from 'zod';
import { NotFoundError } from '@promptsheon/shared';
import type { TeamRepo, SsoConfigRepo } from '../repos/team.js';
import type { AuditChain } from '../audit/chain.js';
import type { UserRepo } from '../repos/user.js';
import type { MembershipRepo } from '../repos/org.js';
import type { VaultRepo } from '../repos/vault.js';
import { parseBody, parseParams, parseQuery } from './validate.js';

function orgOf(request: FastifyRequest): string | null {
  return request.orgContext?.orgId ?? request.agentOrgId ?? null;
}

function actorRole(request: FastifyRequest): string {
  return request.orgContext?.role ?? 'reader';
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

const ScimListQuerySchema = z.object({
  startIndex: z.coerce.number().int().min(1).default(1),
  count: z.coerce.number().int().min(1).max(200).default(50),
});

const ScimPatchSchema = z.object({
  Operations: z.array(z.object({
    op: z.enum(['add', 'replace', 'remove']),
    path: z.string().optional(),
    value: z.unknown().optional(),
  })).min(1),
});

const TeamParamsSchema = z.object({
  id: z.string().trim().min(1).max(255),
});

const TeamMemberParamsSchema = z.object({
  teamId: z.string().trim().min(1).max(255),
  userId: z.string().trim().min(1).max(255),
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
  deps: {
    teamRepo: TeamRepo;
    ssoConfigRepo: SsoConfigRepo;
    auditChain: AuditChain;
    scimBearerToken: string;
    userRepo?: UserRepo;
    membershipRepo?: MembershipRepo;
    vaultRepo?: VaultRepo;
  },
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
      userId: request.userId ?? 'system',
      action: 'team.create',
      resource: 'team',
      details: JSON.stringify({ teamId: team.id, slug: team.slug }),
      resourceKind: 'team',
      resourceId: team.id,
    });
    return reply.code(201).send(team);
  });

  app.post('/api/teams/:id/members', async (request, reply) => {
    const parsedParams = parseParams(reply, TeamParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const orgId = orgOf(request);
    if (!orgId) return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT' } });
    const parsed = parseBody(reply, AddMemberSchema, request.body);
    if (!parsed.ok) return;
    if (actorRole(request) !== 'admin') return reply.code(403).send({ error: { code: 'INSUFFICIENT_ROLE' } });
    const member = deps.teamRepo.addMemberInOrg(id, orgId, parsed.data.userId, parsed.data.role);
    if (!member) return reply.code(404).send({ error: { code: 'NOT_FOUND' } });
    deps.auditChain.append({
      userId: request.userId ?? 'system',
      action: 'team.add_member',
      resource: 'team_member',
      details: JSON.stringify({ teamId: id, userId: parsed.data.userId, role: parsed.data.role }),
      resourceKind: 'team_member',
      resourceId: `${id}:${parsed.data.userId}`,
    });
    return reply.code(201).send(member);
  });

  app.delete('/api/teams/:teamId/members/:userId', async (request, reply) => {
    const parsedParams = parseParams(reply, TeamMemberParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { teamId, userId } = parsedParams.data;
    const orgId = orgOf(request);
    if (!orgId) return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT' } });
    if (actorRole(request) !== 'admin') return reply.code(403).send({ error: { code: 'INSUFFICIENT_ROLE' } });
    const ok = deps.teamRepo.removeMemberInOrg(teamId, orgId, userId);
    if (!ok) return reply.code(404).send({ error: { code: 'NOT_FOUND' } });
    deps.auditChain.append({
      userId: request.userId ?? 'system',
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
    if (!deps.vaultRepo) {
      return reply.code(503).send({
        error: { code: 'OIDC_NOT_CONFIGURED', message: 'OIDC secret storage is not configured' },
      });
    }
    deps.vaultRepo.set(
      orgId,
      'oidc-client-secret',
      parsed.data.clientSecret,
      request.userId ?? 'system',
    );
    deps.ssoConfigRepo.upsert({
      organizationId: orgId,
      provider: parsed.data.provider,
      issuer: parsed.data.issuer,
      clientId: parsed.data.clientId,
      clientSecretEncrypted: `vault://${orgId}/oidc-client-secret`,
      scopes: parsed.data.scopes,
      audience: parsed.data.audience,
      groupsClaim: parsed.data.groupsClaim,
      emailClaim: parsed.data.emailClaim,
      nameClaim: parsed.data.nameClaim,
    });
    deps.auditChain.append({
      userId: request.userId ?? 'system',
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
    const expected = createHash('sha256').update(deps.scimBearerToken).digest();
    const actual = createHash('sha256').update(presented).digest();
    if (expected.length !== actual.length || !timingSafeEqual(expected, actual)) {
      reply.code(401).send({ error: { code: 'UNAUTHORIZED' } });
      return false;
    }
    return true;
  };

  app.get('/api/scim/v2/Users', async (request, reply) => {
    if (!(await scimAuth(request, reply))) return;
    const orgId = orgOf(request);
    if (!orgId) return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT' } });
    if (!deps.userRepo || !deps.membershipRepo) {
      return reply.code(503).send({ error: { code: 'SCIM_NOT_CONFIGURED', message: 'SCIM persistence is not configured' } });
    }
    const parsed = parseQuery(reply, ScimListQuerySchema, request.query);
    if (!parsed.ok) return;
    const allUsers = deps.userRepo.listForOrg(orgId);
    const rows = allUsers.slice(parsed.data.startIndex - 1, parsed.data.startIndex - 1 + parsed.data.count);
    return reply.send({
      schemas: ['urn:ietf:params:scim:api:messages:2.0:ListResponse'],
      totalResults: allUsers.length,
      startIndex: parsed.data.startIndex,
      itemsPerPage: rows.length,
      Resources: rows.map((user) => toScimUser(user)),
    });
  });

  app.post('/api/scim/v2/Users', async (request, reply) => {
    if (!(await scimAuth(request, reply))) return;
    const orgId = orgOf(request);
    if (!orgId) return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT' } });
    const parsed = parseBody(reply, ScimUserSchema, request.body);
    if (!parsed.ok) return;
    if (!deps.userRepo || !deps.membershipRepo) {
      return reply.code(503).send({ error: { code: 'SCIM_NOT_CONFIGURED', message: 'SCIM persistence is not configured' } });
    }
    const email = parsed.data.emails.find((e) => e.primary)?.value ?? parsed.data.emails[0]?.value;
    if (!email) {
      return reply.code(400).send({ error: { code: 'MISSING_EMAIL', message: 'at least one email required' } });
    }
    const existing = deps.userRepo.findByEmail(email);
    const user = existing ?? deps.userRepo.create({ email, name: parsed.data.displayName ?? parsed.data.userName, role: 'reader' });
    const memberRole = parsed.data.roles.some((role) => role.toLowerCase() === 'admin') ? 'admin' : 'viewer';
    deps.membershipRepo.addOrgMember(orgId, user.id, memberRole);
    deps.auditChain.append({
      userId: user.id,
      action: 'scim.user_create',
      resource: 'user',
      details: JSON.stringify({ userName: parsed.data.userName, email }),
      resourceKind: 'user',
      resourceId: user.id,
    });
    return reply.code(existing ? 200 : 201).send(toScimUser(user));
  });

  app.patch('/api/scim/v2/Users/:id', async (request, reply) => {
    if (!(await scimAuth(request, reply))) return;
    const orgId = orgOf(request);
    if (!orgId) return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT' } });
    if (!deps.userRepo || !deps.membershipRepo) {
      return reply.code(503).send({ error: { code: 'SCIM_NOT_CONFIGURED', message: 'SCIM persistence is not configured' } });
    }
    const parsed = parseBody(reply, ScimPatchSchema, request.body);
    if (!parsed.ok) return;
    const parsedParams = parseParams(reply, TeamParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const user = deps.userRepo.findByIdInOrg(id, orgId);
    if (!user) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'SCIM user not found' } });
    for (const operation of parsed.data.Operations) {
      if ((operation.path ?? '').toLowerCase() !== 'active') continue;
      const value = typeof operation.value === 'boolean' ? operation.value : undefined;
      if (value === false || operation.op === 'remove') deps.membershipRepo.removeOrgMember(orgId, id);
      if (value === true) deps.membershipRepo.addOrgMember(orgId, id, 'viewer');
    }
    return reply.send(toScimUser(user, deps.membershipRepo.findOrgMembers(orgId).some((member) => member.userId === id)));
  });

  app.delete('/api/scim/v2/Users/:id', async (request, reply) => {
    if (!(await scimAuth(request, reply))) return;
    const orgId = orgOf(request);
    if (!orgId) return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT' } });
    if (!deps.membershipRepo) {
      return reply.code(503).send({ error: { code: 'SCIM_NOT_CONFIGURED', message: 'SCIM persistence is not configured' } });
    }
    const parsedParams = parseParams(reply, TeamParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    if (!deps.membershipRepo.removeOrgMember(orgId, id)) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'SCIM user not found' } });
    }
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

function toScimUser(user: { id: string; email: string; name: string; createdAt: string }, active = true): Record<string, unknown> {
  return {
    schemas: ['urn:ietf:params:scim:schemas:core:2.0:User'],
    id: user.id,
    userName: user.email,
    displayName: user.name,
    emails: [{ value: user.email, primary: true }],
    active,
    meta: { resourceType: 'User', created: user.createdAt },
  };
}
