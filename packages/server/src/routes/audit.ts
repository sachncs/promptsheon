import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type { AuditChain } from '../audit/chain.js';
import { applyMigrations } from '@promptsheon/shared';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const ListQuerySchema = z.object({
  resource: z.string().optional(),
  action: z.string().optional(),
  limit: z.coerce.number().int().min(1).max(500).optional().default(100),
  offset: z.coerce.number().int().min(0).optional().default(0),
});

const VerifyQuerySchema = z.object({});

/**
 * Register audit-trail HTTP routes. Returns the immutable chain
 * (oldest first) and exposes verify() for tamper checks.
 */
export function registerAuditRoutes(app: FastifyInstance, deps: { auditChain: AuditChain; db: import('better-sqlite3').Database }) {
  app.get('/api/audit', async (request, reply) => {
    const params = request.query as Record<string, string | undefined>;
    const limit = Math.min(parseInt(params['limit'] ?? '100', 10) || 100, 500);
    const offset = parseInt(params['offset'] ?? '0', 10) || 0;
    const resource = params['resource'];
    const action = params['action'];

    const where: string[] = [];
    const args: unknown[] = [];
    if (resource) { where.push('resource = ?'); args.push(resource); }
    if (action) { where.push('action = ?'); args.push(action); }
    const whereClause = where.length > 0 ? `WHERE ${where.join(' AND ')}` : '';
    const rows = deps.db
      .prepare(
        `SELECT id, user_id AS userId, action, resource, details, timestamp,
                previous_hash AS previousHash, entry_hash AS entryHash,
                timestamp_str AS timestampStr,
                resource_kind AS resourceKind, resource_id AS resourceId
         FROM audit_entries ${whereClause}
         ORDER BY rowid DESC LIMIT ? OFFSET ?`,
      )
      .all(...args, limit, offset) as Array<{
      id: string;
      userId: string;
      action: string;
      resource: string;
      details: string;
      timestamp: string;
      previousHash: string;
      entryHash: string;
      timestampStr: string;
      resourceKind: string;
      resourceId: string;
    }>;

    return reply.send({ entries: rows, limit, offset });
  });

  app.get('/api/audit/verify', async (_request, reply) => {
    const result = deps.auditChain.verify();
    return reply.send(result);
  });

  app.get('/api/audit/state', async (_request, reply) => {
    return reply.send(deps.auditChain.getChainState());
  });
}