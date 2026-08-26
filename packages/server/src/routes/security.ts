import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type { PromptScanRepo } from '../repos/prompt-scan.js';
import { scan } from '../security/prompt-scanner.js';
import { parseBody, parseQuery } from './validate.js';

const ScanTextSchema = z.object({
  text: z.string().min(1).max(64_000),
  resourceKind: z.string().min(1).max(60).optional(),
  resourceId: z.string().min(1).max(120).optional(),
});

interface RequestUserContext {
  userId?: string;
  orgContext?: { organizationId?: string };
}

interface RequestLike {
  userId?: string;
  orgContext?: { organizationId?: string };
  headers: Record<string, string | string[] | undefined>;
}

function orgOf(request: unknown): string | null {
  const req = request as RequestLike | undefined;
  if (!req) return null;
  if (req.orgContext?.organizationId) return req.orgContext.organizationId;
  const raw = req.headers['x-org-id'];
  if (typeof raw === 'string') return raw;
  if (Array.isArray(raw) && typeof raw[0] === 'string') return raw[0];
  return null;
}

function actorOf(request: unknown): string | null {
  const req = request as RequestLike | undefined;
  if (!req) return null;
  if (req.userId) return req.userId;
  const raw = req.headers['x-user-id'];
  if (typeof raw === 'string') return raw;
  if (Array.isArray(raw) && typeof raw[0] === 'string') return raw[0];
  return null;
}

const ListScansQuerySchema = z.object({
  days: z.coerce.number().int().min(1).max(365).default(30),
  verdict: z.enum(['clean', 'warn', 'block']).optional(),
  resourceKind: z.string().min(1).max(60).optional(),
  resourceId: z.string().min(1).max(120).optional(),
});

/**
 * T2-3 security surface.
 *   POST /api/security/scan              — run the static scanner
 *     over arbitrary text. Returns the verdict + findings list
 *     WITHOUT persisting (caller decides whether to persist).
 *   POST /api/security/scan-and-save      — scan + persist as a
 *     prompt_scans row tagged to the (resourceKind, resourceId).
 *   GET  /api/security/scans             — org-wide scan history.
 *   GET  /api/security/scans/summary      — verdict breakdown.
 */
export function registerSecurityRoutes(
  app: FastifyInstance,
  deps: { scanRepo: PromptScanRepo },
) {
  app.post('/api/security/scan', async (request, reply) => {
    const parsed = parseBody(reply, ScanTextSchema, request.body);
    if (!parsed.ok) return;
    const result = scan({ text: parsed.data.text });
    return reply.send(result);
  });

  app.post('/api/security/scan-and-save', async (request, reply) => {
    const parsed = parseBody(reply, ScanTextSchema, request.body);
    if (!parsed.ok) return;
    const orgId = orgOf(request);
    if (!orgId) {
      return reply.code(401).send({
        error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' },
      });
    }
    const result = scan({ text: parsed.data.text });
    const resourceKind = parsed.data.resourceKind ?? 'ad-hoc';
    const resourceId = parsed.data.resourceId ?? `adhoc-${Date.now()}`;
    const saved = deps.scanRepo.record({
      organizationId: orgId,
      actorId: actorOf(request),
      resourceKind,
      resourceId,
      verdict: result.verdict,
      findings: result.findings,
    });
    return reply.send({ ...result, scan: saved });
  });

  app.get('/api/security/scans', async (request, reply) => {
    const orgId = orgOf(request);
    if (!orgId) {
      return reply.code(401).send({
        error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' },
      });
    }
    const parsed = parseQuery(reply, ListScansQuerySchema, request.query);
    if (!parsed.ok) return;
    const { days, verdict } = parsed.data;
    const items = deps.scanRepo.listByOrg(orgId, { days, verdict });
    return reply.send({ items, total: items.length });
  });

  app.get('/api/security/scans/summary', async (request, reply) => {
    const orgId = orgOf(request);
    if (!orgId) {
      return reply.code(401).send({
        error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' },
      });
    }
    const days = Number((request.query as { days?: string }).days ?? '30');
    const summary = deps.scanRepo.summaryByOrg(orgId, Math.min(Math.max(days, 1), 365));
    return reply.send({ orgId, days, ...summary });
  });
}
