import type { FastifyInstance, FastifyReply, FastifyRequest } from 'fastify';
import { z } from 'zod';
import type { EvidenceEventType, EvidenceRepo } from '../repos/evidence.js';
import { parseParams, parseQuery } from './validate.js';

const EvidenceQuerySchema = z.strictObject({
  limit: z.coerce.number().int().min(1).max(500).default(100),
  before: z.string().datetime({ offset: true }).optional(),
  eventType: z.string().min(1).max(80).optional(),
  agentHash: z.string().regex(/^[0-9a-f]{64}$/).optional(),
  workspaceId: z.string().min(1).max(255).optional(),
  traceId: z.string().min(1).max(255).optional(),
});
const TraceParamsSchema = z.strictObject({ id: z.string().min(1).max(255) });
const TraceEvidenceQuerySchema = z.strictObject({ workspaceId: z.string().min(1).max(255).optional() });

function organizationOf(request: FastifyRequest): string | null {
  return request.orgContext?.orgId ?? request.agentOrgId ?? null;
}

/** Tenant-scoped evidence timeline and export adapter. */
export function registerEvidenceRoutes(
  app: FastifyInstance,
  deps: { repo: EvidenceRepo; requireAdmin: () => (request: FastifyRequest, reply: FastifyReply) => Promise<void> },
): void {
  app.get('/api/evidence', { preHandler: deps.requireAdmin() }, async (request, reply) => {
    const organizationId = organizationOf(request);
    if (!organizationId) return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    const parsed = parseQuery(reply, EvidenceQuerySchema, request.query);
    if (!parsed.ok) return;
    const { limit, before, eventType, traceId, agentHash, workspaceId } = parsed.data;
    if (traceId) {
      const items = deps.repo.listByTrace(organizationId, traceId, workspaceId);
      return reply.send({ items, total: items.length });
    }
    const items = deps.repo.listByOrganization(organizationId, {
      limit,
      ...(before ? { before } : {}),
      ...(eventType ? { eventType: eventType as EvidenceEventType } : {}),
      ...(agentHash ? { agentHash } : {}),
      ...(workspaceId ? { workspaceId } : {}),
    });
    return reply.send({ items, total: items.length });
  });

  app.get('/api/evidence/export', { preHandler: deps.requireAdmin() }, async (request, reply) => {
    const organizationId = organizationOf(request);
    if (!organizationId) return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    const parsed = parseQuery(reply, EvidenceQuerySchema, request.query);
    if (!parsed.ok) return;
    const items = parsed.data.traceId
      ? deps.repo.listByTrace(organizationId, parsed.data.traceId, parsed.data.workspaceId)
      : deps.repo.listByOrganization(organizationId, { limit: parsed.data.limit, ...(parsed.data.before ? { before: parsed.data.before } : {}), ...(parsed.data.eventType ? { eventType: parsed.data.eventType as EvidenceEventType } : {}), ...(parsed.data.agentHash ? { agentHash: parsed.data.agentHash } : {}), ...(parsed.data.workspaceId ? { workspaceId: parsed.data.workspaceId } : {}) });
    reply.header('Content-Disposition', 'attachment; filename="promptsheon-evidence.json"');
    return reply.send({ schemaVersion: '1.0', organizationId, exportedAt: new Date().toISOString(), items });
  });

  app.get('/api/traces/:id/evidence', { preHandler: deps.requireAdmin() }, async (request, reply) => {
    const parsed = parseParams(reply, TraceParamsSchema, request.params);
    if (!parsed.ok) return;
    const organizationId = organizationOf(request);
    if (!organizationId) return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    const query = parseQuery(reply, TraceEvidenceQuerySchema, request.query);
    if (!query.ok) return;
    const items = deps.repo.listByTrace(organizationId, parsed.data.id, query.data.workspaceId);
    return reply.send({ traceId: parsed.data.id, items, total: items.length });
  });
}
