import type { FastifyInstance, FastifyRequest } from 'fastify';
import { z } from 'zod';
import type { AuditChain } from '../audit/chain.js';
import type { AuditReplicationService } from '../application/audit-replication-service.js';
import { parseQuery } from './validate.js';

const ListQuerySchema = z.object({
  resource: z.string().max(120).optional(),
  action: z.string().max(120).optional(),
  limit: z.coerce.number().int().min(1).max(500).optional().default(100),
  offset: z.coerce.number().int().min(0).optional().default(0),
});

const VerifyQuerySchema = z.object({});
const AuditReplicationFrameSchema = z.object({
  rowid: z.number().int().nonnegative(),
  previousHash: z.string(),
  entry: z.object({
    id: z.string().min(1),
    userId: z.string().min(1),
    action: z.string().min(1),
    resource: z.string().min(1),
    details: z.string(),
    timestamp: z.string().datetime(),
    entryHash: z.string().min(1),
    resourceKind: z.string().min(1),
    resourceId: z.string().min(1),
  }),
});

/**
 * Register audit-trail HTTP routes. Returns the immutable chain
 * (oldest first) and exposes verify() for tamper checks.
 */
function organizationOf(request: FastifyRequest): string | undefined {
  return request.orgContext?.orgId ?? request.agentOrgId;
}

export function registerAuditRoutes(
  app: FastifyInstance,
  deps: { auditChain: AuditChain; replication: AuditReplicationService },
) {
  app.get('/api/audit', async (request, reply) => {
    const organizationId = organizationOf(request);
    if (!organizationId) {
      return reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
    }
    const parsed = parseQuery(reply, ListQuerySchema, request.query);
    if (!parsed.ok) return;
    const { limit, offset, resource, action } = parsed.data;

    const rows = deps.auditChain
      .entriesForOrganization(organizationId)
      .filter((entry) => !resource || entry.resource === resource)
      .filter((entry) => !action || entry.action === action)
      .reverse()
      .slice(offset, offset + limit);

    return reply.send({ entries: rows, limit, offset });
  });

  app.get('/api/audit/verify', async (_request, reply) => {
    const result = deps.auditChain.verify();
    return reply.send(result);
  });

  app.get('/api/audit/state', async (_request, reply) => {
    return reply.send(deps.auditChain.getChainState());
  });

  /**
   * Ingest an audit frame from the replicator daemon.
   * Idempotent on `id` so retries from the primary never duplicate
   * rows on the replica. The replica treats this as the source
   * of truth for the audit log — anything the primary ships here
   * becomes the canonical entry on this server.
   *
   * Wired only on hosts running as a replica; the primary does
   * not need this route.
   */
  app.post('/api/audit/ingest', async (request, reply) => {
    const parsed = AuditReplicationFrameSchema.safeParse(request.body);
    if (!parsed.success) {
      return reply.code(400).send({
        error: { code: 'INVALID_FRAME', message: 'malformed audit frame' },
      });
    }
    return reply.send({ ok: true, ...deps.replication.ingest(parsed.data) });
  });

  /**
   * Returns the highest rowid the replica has stored so the
   * primary's replicator daemon knows where to resume from.
   */
  app.get('/api/audit/replication-state', async (_request, reply) => {
    const state = deps.auditChain.getChainState();
    return reply.send({ lastRowid: state.lastRowid, lastHash: state.lastHash });
  });
}
