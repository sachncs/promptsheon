import type { FastifyInstance, FastifyReply, FastifyRequest } from 'fastify';
import { z } from 'zod';
import { parseBody, parseParams, parseQuery } from './validate.js';
import type { VaultRepo, Kms } from '../repos/vault.js';
import type { OrgExportService } from '../repos/vault-extras.js';
import type { CostRollupRepo } from '../repos/vault-extras.js';
import type { SearchRepo } from '../repos/search.js';

const VaultSetSchema = z.object({
  organizationId: z.string(),
  name: z.string().min(1).max(120).regex(/^[A-Za-z0-9._\-/]+$/),
  value: z.string().min(1),
});

const ExportRequestSchema = z.object({
  organizationId: z.string(),
});

const PurgeRequestSchema = z.object({
  organizationId: z.string(),
});

const CostQuerySchema = z.object({
  organizationId: z.string(),
  days: z.coerce.number().int().min(1).max(365).optional(),
});

const SecretsQuerySchema = z.object({
  organizationId: z.string().min(1).max(200),
});

const SearchQuerySchema = z.object({
  q: z.string().max(500).optional(),
  type: z.string().min(1).max(80).optional(),
});

const RollupIngestSchema = z.object({
  capabilityId: z.string(),
  input: z.number().int().min(0).optional(),
  output: z.number().int().min(0).optional(),
  costMicros: z.number().int().min(0).optional(),
  executions: z.number().int().min(0).optional(),
});

const RotateKeySchema = z.object({
  label: z.string().min(1).max(120),
  reencrypt: z.boolean().optional(),
});

const OrganizationParamsSchema = z.object({
  id: z.string().trim().min(1).max(255),
});

export interface VaultRouteDeps {
  vaultRepo: VaultRepo;
  orgExportService: OrgExportService;
  costRollupRepo: CostRollupRepo;
  searchRepo: SearchRepo;
  kms: Kms;
  adminOnly: (request: FastifyRequest) => boolean;
}

function activeOrg(request: FastifyRequest): string | undefined {
  return request.orgContext?.orgId ?? request.agentOrgId;
}

function assertOrgScope(
  request: FastifyRequest,
  requestedOrgId: string,
  reply: FastifyReply,
): boolean {
  const current = activeOrg(request);
  if (current && current !== requestedOrgId) {
    void reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'organization not found' } });
    return false;
  }
  return true;
}

function actorOf(request: FastifyRequest): string {
  return request.userId ?? 'system';
}

function escapeFts(s: string): string {
  return s.replace(/[\u0000-\u001f]/g, ' ').split(/\s+/).filter(Boolean).map((w) => `${w}*`).join(' ');
}

export function registerVaultRoutes(app: FastifyInstance, deps: VaultRouteDeps): void {
  // Vault
  app.get('/api/vault/secrets', async (request, reply) => {
    const parsed = parseQuery(reply, SecretsQuerySchema, request.query);
    if (!parsed.ok) return;
    const { organizationId } = parsed.data;
    if (!assertOrgScope(request, organizationId, reply)) return;
    return reply.send(deps.vaultRepo.list(organizationId));
  });

  app.post('/api/vault/secrets', async (request, reply) => {
    if (!deps.adminOnly(request)) {
      return reply.code(403).send({ error: { code: 'FORBIDDEN', message: 'admin only' } });
    }
    const parsed = parseBody(reply, VaultSetSchema, request.body);
    if (!parsed.ok) return;
    if (!assertOrgScope(request, parsed.data.organizationId, reply)) return;
    const created = deps.vaultRepo.set(
      parsed.data.organizationId,
      parsed.data.name,
      parsed.data.value,
      actorOf(request),
    );
    return reply.code(201).send(created);
  });

  // Keyring
  app.get('/api/vault/keys', async (request, reply) => {
    if (!deps.adminOnly(request)) {
      return reply.code(403).send({ error: { code: 'FORBIDDEN', message: 'admin only' } });
    }
    return reply.send(deps.vaultRepo.listKeyring());
  });

  app.post('/api/vault/keys/rotate', async (request, reply) => {
    if (!deps.adminOnly(request)) {
      return reply.code(403).send({ error: { code: 'FORBIDDEN', message: 'admin only' } });
    }
    const parsed = parseBody(reply, RotateKeySchema, request.body);
    if (!parsed.ok) return;
    const current = deps.vaultRepo.listKeyring().find((k) => k.active);
    if (!current) {
      return reply.code(500).send({ error: { code: 'NO_ACTIVE_KEY', message: 'no active encryption key' } });
    }
    const next = deps.vaultRepo.rotate(current.fingerprint, parsed.data.label);
    let re = 0;
    if (parsed.data.reencrypt !== false) {
      re = deps.vaultRepo.reencryptAllFromKey(current.fingerprint, next.fingerprint);
    }
    return reply.send({ key: next, reencrypted: re });
  });

  // Export + purge
  app.post('/api/orgs/:id/export', async (request, reply) => {
    if (!deps.adminOnly(request)) {
      return reply.code(403).send({ error: { code: 'FORBIDDEN', message: 'admin only' } });
    }
    const parsedParams = parseParams(reply, OrganizationParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    if (!assertOrgScope(request, id, reply)) return;
    const exp = await deps.orgExportService.exportAll(id, actorOf(request));
    deps.orgExportService.recordExport(exp);
    return reply.code(202).send(exp);
  });

  app.post('/api/orgs/:id/purge', async (request, reply) => {
    if (!deps.adminOnly(request)) {
      return reply.code(403).send({ error: { code: 'FORBIDDEN', message: 'admin only' } });
    }
    const parsedParams = parseParams(reply, OrganizationParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    if (!assertOrgScope(request, id, reply)) return;
    const result = deps.orgExportService.schedulePurge(id, actorOf(request));
    return reply.send(result);
  });

  // Cost / analytics
  app.post('/api/analytics/rollups', async (request, reply) => {
    const organizationId = activeOrg(request);
    if (!organizationId) {
      return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    }
    const parsed = parseBody(reply, RollupIngestSchema, request.body);
    if (!parsed.ok) return;
    if (!deps.costRollupRepo.capabilityBelongsToOrg(parsed.data.capabilityId, organizationId)) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'capability not found' } });
    }
    const today = new Date().toISOString().slice(0, 10);
    deps.costRollupRepo.record(
      parsed.data.capabilityId,
      today,
      parsed.data.input ?? 0,
      parsed.data.output ?? 0,
      parsed.data.costMicros ?? 0,
      parsed.data.executions ?? 0,
    );
    return reply.code(204).send();
  });

  app.get('/api/analytics/cost', async (request, reply) => {
    const parsed = parseQuery(reply, CostQuerySchema, request.query);
    if (!parsed.ok) return;
    if (!assertOrgScope(request, parsed.data.organizationId, reply)) return;
    return reply.send(deps.costRollupRepo.rollupsForOrg(parsed.data.organizationId, parsed.data.days ?? 30));
  });

  // Search (FTS5)
  app.get('/api/search', async (request, reply) => {
    const parsed = parseQuery(reply, SearchQuerySchema, request.query);
    if (!parsed.ok) return;
    const { q, type } = parsed.data;
    if (!q || q.length < 2) return reply.send([]);
    return reply.send(deps.searchRepo.search(escapeFts(q), type));
  });
}
