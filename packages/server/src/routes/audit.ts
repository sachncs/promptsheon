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
    const frame = request.body as {
      rowid: number;
      previousHash: string;
      entry: {
        id: string;
        userId: string;
        action: string;
        resource: string;
        details: string;
        timestamp: string;
        entryHash: string;
        resourceKind: string;
        resourceId: string;
      };
    };
    if (
      typeof frame !== 'object' ||
      frame === null ||
      typeof frame.rowid !== 'number' ||
      typeof frame.previousHash !== 'string' ||
      !frame.entry ||
      typeof frame.entry.id !== 'string'
    ) {
      return reply.code(400).send({
        error: { code: 'INVALID_FRAME', message: 'malformed audit frame' },
      });
    }
    const insert = deps.db.prepare(
      `INSERT OR IGNORE INTO audit_entries
         (id, user_id, action, resource, details, timestamp, previous_hash, entry_hash, timestamp_str, resource_kind, resource_id)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    );
    const result = insert.run(
      frame.entry.id,
      frame.entry.userId,
      frame.entry.action,
      frame.entry.resource,
      frame.entry.details,
      frame.entry.timestamp,
      frame.previousHash,
      frame.entry.entryHash,
      frame.entry.timestamp,
      frame.entry.resourceKind,
      frame.entry.resourceId,
    );
    if (result.changes > 0) {
      // Update the chain state — only advance if we just inserted
      // a new row. Duplicate inserts leave the state alone.
      deps.db
        .prepare(
          `UPDATE audit_chain_state
           SET last_hash = ?, last_rowid = ?
           WHERE id = 0 AND last_rowid < ?`,
        )
        .run(frame.entry.entryHash, frame.rowid, frame.rowid);
    }
    return reply.send({ ok: true, duplicate: result.changes === 0 });
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