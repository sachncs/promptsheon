import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { parseBody } from './validate.js';
import type { VaultRepo } from '../repos/vault.js';
import type { OrgExportService } from '../repos/vault-extras.js';
import type { CostRollupRepo } from '../repos/vault-extras.js';

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

const RollupIngestSchema = z.object({
  capabilityId: z.string(),
  input: z.number().int().min(0).optional(),
  output: z.number().int().min(0).optional(),
  costMicros: z.number().int().min(0).optional(),
  executions: z.number().int().min(0).optional(),
});

export interface VaultRouteDeps {
  vaultRepo: VaultRepo;
  orgExportService: OrgExportService;
  costRollupRepo: CostRollupRepo;
  adminOnly: (request: unknown) => boolean;
}

function actorOf(request: unknown): string {
  const ctx = (request as { userId?: string } | undefined) ?? {};
  return ctx.userId ?? 'system';
}

export function registerVaultRoutes(app: FastifyInstance, deps: VaultRouteDeps): void {
  // Vault
  app.get('/api/vault/secrets', async (request, reply) => {
    const { organizationId } = request.query as { organizationId?: string };
    if (!organizationId) {
      return reply.code(400).send({ error: { code: 'BAD_REQUEST', message: 'organizationId required' } });
    }
    return reply.send(deps.vaultRepo.list(organizationId));
  });

  app.post('/api/vault/secrets', async (request, reply) => {
    if (!deps.adminOnly(request)) {
      return reply.code(403).send({ error: { code: 'FORBIDDEN', message: 'admin only' } });
    }
    const parsed = parseBody(reply, VaultSetSchema, request.body);
    if (!parsed.ok) return;
    const created = deps.vaultRepo.set(
      parsed.data.organizationId,
      parsed.data.name,
      parsed.data.value,
      actorOf(request),
    );
    return reply.code(201).send(created);
  });

  // Export + purge
  app.post('/api/orgs/:id/export', async (request, reply) => {
    if (!deps.adminOnly(request)) {
      return reply.code(403).send({ error: { code: 'FORBIDDEN', message: 'admin only' } });
    }
    const { id } = request.params as { id: string };
    const exp = await deps.orgExportService.exportAll(id, actorOf(request));
    deps.orgExportService.recordExport(exp);
    return reply.code(202).send(exp);
  });

  app.post('/api/orgs/:id/purge', async (request, reply) => {
    if (!deps.adminOnly(request)) {
      return reply.code(403).send({ error: { code: 'FORBIDDEN', message: 'admin only' } });
    }
    const { id } = request.params as { id: string };
    const result = deps.orgExportService.schedulePurge(id, actorOf(request));
    return reply.send(result);
  });

  // Cost / analytics
  app.post('/api/analytics/rollups', async (request, reply) => {
    const parsed = parseBody(reply, RollupIngestSchema, request.body);
    if (!parsed.ok) return;
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
    const parsed = parseQuerySchema(reply, request.query);
    if (!parsed.ok) return;
    return reply.send(deps.costRollupRepo.rollupsForOrg(parsed.data.organizationId, parsed.data.days ?? 30));
  });

  // Search (FTS5)
  app.get('/api/search', async (request, reply) => {
    const { q, type } = request.query as { q?: string; type?: string };
    if (!q || q.length < 2) return reply.send([]);
    const where = type ? 'AND kind = ?' : '';
    const params: unknown[] = [escapeFts(q)];
    if (type) params.push(type);
    const stmt = deps.costRollupRepo['db'].prepare.bind(deps.costRollupRepo['db']);
    void stmt;
    // We use CostRollupRepo.db; read directly here. Alternative would be
    // a dedicated SearchRepo; defer that until second pass.
    const rows = (deps.costRollupRepo as unknown as { db: { prepare: (s: string) => { all: (...p: unknown[]) => Array<{ kind: string; resource_id: string; title: string; body: string }> } } })
      .db
      .prepare(`SELECT kind, resource_id, title, body FROM search_index WHERE search_index MATCH ? ${where} ORDER BY rank LIMIT 50`)
      .all(...params);
    return reply.send(rows);
  });
}

function parseQuerySchema(
  reply: { code: (n: number) => { send: (p: unknown) => void } },
  query: unknown,
): { ok: true; data: { organizationId: string; days?: number } } | { ok: false } {
  const schema = z.object({
    organizationId: z.string(),
    days: z.coerce.number().int().min(1).max(365).optional(),
  });
  const result = schema.safeParse(query);
  if (result.success) return { ok: true, data: result.data };
  reply.code(422).send({
    error: { code: 'VALIDATION_ERROR', message: 'Query validation failed' },
  });
  return { ok: false };
}

function escapeFts(s: string): string {
  return s.replace(/[\u0000-\u001f]/g, ' ').split(/\s+/).filter(Boolean).map((w) => `${w}*`).join(' ');
}
