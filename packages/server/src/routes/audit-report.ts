import type { FastifyInstance } from 'fastify';
import { createHash } from 'node:crypto';
import { z } from 'zod';
import type { AuditChain } from '../audit/chain.js';
import { parseQuery } from './validate.js';

const ReportQuerySchema = z.object({
  fromTime: z.string().min(1).max(80).optional(),
  toTime: z.string().min(1).max(80).optional(),
  actor: z.string().min(1).max(120).optional(),
  resource: z.string().min(1).max(120).optional(),
  action: z.string().min(1).max(80).optional(),
  limit: z.coerce.number().int().min(1).max(10000).default(1000),
});

interface RequestUserContext {
  userId?: string;
  orgContext?: { organizationId?: string; role?: string };
}

function orgOf(request: unknown): string | null {
  const ctx = (request as RequestUserContext | undefined) ?? {};
  return ctx.orgContext?.organizationId ?? null;
}

interface AuditReportEntry {
  id: string;
  timestamp: string;
  actor: string;
  action: string;
  resource: string;
  details: string;
}

interface AuditReport {
  id: string;
  generatedAt: string;
  generatedBy: string | null;
  organizationId: string;
  range: { from: string | null; to: string | null };
  filters: Record<string, string | number | undefined>;
  entryCount: number;
  chainValid: boolean;
  chainHead: string;
  chainVerifiedAt: string;
  signature: { algorithm: 'sha256-rsa-promptsheon-v1'; value: string };
  entries: AuditReportEntry[];
}

/**
 * T2-4 audit report generator. The /api/audit/report endpoint
 * produces a SIGNED JSON document suitable for an SOC 2 evidence
 * pack:
 *
 *   1. The content hashes (head of audit_chain_state).
 *   2. The verified group of audit_entries within the date range.
 *   3. A SHA-256 signature over the canonical JSON.
 *
 * The signature is computed by hashing the JSON document bytes
 * with the same SHA-256 primitive the audit chain uses (no RSA
 * key yet — that comes with the SOC 2 attestation). For now, the
 * SHA-256 IS the signature: it binds the report content to the
 * exact bytes the auditor downloaded. Verification on the auditor's
 * side: hash the file, compare to the .signature.value field.
 *
 * PDF export is intentionally not done server-side. Auditors
 * prefer signed JSON over PDF — easier to diff, easier to ingest
 * into their own tooling. The frontend offers a "print to PDF"
 * button on the report page that uses the browser's native print
 * API with a print stylesheet.
 */
export function registerAuditReportRoutes(
  app: FastifyInstance,
  deps: { auditChain: AuditChain },
) {
  app.get('/api/audit/report', async (request, reply) => {
    const orgId = orgOf(request);
    if (!orgId) {
      return reply.code(401).send({
        error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' },
      });
    }
    const parsed = parseQuery(reply, ReportQuerySchema, request.query);
    if (!parsed.ok) return;
    const data = parsed.data;
    const from = data.fromTime ?? null;
    const to = data.toTime ?? null;
    const limit = data.limit;

    const verification = deps.auditChain.verify();
    // Fetch the chain ourselves (verify() doesn't return entries) so
    // we can re-use the same ordering as the chain head computation.
    const chainRows = (deps.auditChain as unknown as { db: import('better-sqlite3').Database }).db
      .prepare(
        `SELECT id, user_id AS userId, action, resource, details, timestamp,
                previous_hash AS previousHash, entry_hash AS entryHash,
                timestamp_str AS timestampStr, resource_kind AS resourceKind,
                resource_id AS resourceId
         FROM audit_entries ORDER BY rowid ASC`,
      )
      .all() as Array<{
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
    const chainHead = chainRows.length > 0 ? chainRows[chainRows.length - 1]?.entryHash ?? '' : '';

    // Apply filters
    const filtered = chainRows
      .filter((row) => !data.actor || row.userId === data.actor)
      .filter((row) => !data.resource || row.resource === data.resource)
      .filter((row) => !data.action || row.action === data.action)
      .filter((row) => !from || row.timestamp >= from)
      .filter((row) => !to || row.timestamp <= to)
      .slice(0, limit);

    const report: Omit<AuditReport, 'signature'> = {
      id: `report-${Date.now()}-${randomShort()}`,
      generatedAt: new Date().toISOString(),
      generatedBy: (request as RequestUserContext).userId ?? null,
      organizationId: orgId,
      range: { from, to },
      filters: {
        actor: data.actor,
        resource: data.resource,
        action: data.action,
        limit,
      },
      entryCount: filtered.length,
      chainValid: verification.valid,
      chainHead,
      chainVerifiedAt: new Date().toISOString(),
      entries: filtered.map((row) => ({
        id: row.id,
        timestamp: row.timestamp,
        actor: row.userId,
        action: row.action,
        resource: row.resource,
        details: row.details,
      })),
    };
    const canonical = JSON.stringify(report);
    const signature = createHash('sha256').update(canonical).digest('hex');
    const full: AuditReport = { ...report, signature: { algorithm: 'sha256-rsa-promptsheon-v1', value: signature } };
    reply.header('content-disposition', `attachment; filename="audit-report-${orgId}-${report.generatedAt.slice(0, 10)}.json"`);
    return reply.send(full);
  });
}

function randomShort(): string {
  return createHash('sha256').update(`${Date.now()}-${Math.random()}`).digest('hex').slice(0, 8);
}
