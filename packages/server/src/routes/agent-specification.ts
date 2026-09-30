import type { FastifyInstance, FastifyReply, FastifyRequest } from 'fastify';
import { z } from 'zod';
import { AgentSpecificationSchema } from '@promptsheon/shared';
import type { AgentSpecificationRepo } from '../repos/agent-specification.js';
import type { WorkspaceRepo } from '../repos/workspace.js';
import type { PromptScanRepo } from '../repos/prompt-scan.js';
import { scan } from '../security/prompt-scanner.js';
import { parseBody, parseParams, parseQuery } from './validate.js';

const WorkspaceParams = z.strictObject({ workspaceId: z.string().uuid() });
const HashParams = WorkspaceParams.extend({ hash: z.string().regex(/^[0-9a-f]{64}$/) });
const CreateBody = z.strictObject({
  specification: z.unknown(),
  changeReason: z.string().trim().min(1).max(2000),
  parentHash: z.string().regex(/^[0-9a-f]{64}$/).nullable().optional(),
});
const DiffBody = z.strictObject({
  leftHash: z.string().regex(/^[0-9a-f]{64}$/),
  rightHash: z.string().regex(/^[0-9a-f]{64}$/),
});
const ValidateBody = z.strictObject({ specification: z.unknown() });
const ListQuery = z.object({
  page: z.coerce.number().int().min(1).default(1),
  pageSize: z.coerce.number().int().min(1).max(100).default(20),
  status: z.enum(['draft', 'candidate', 'published', 'retired']).optional(),
});

function organizationIdOf(request: FastifyRequest): string | null {
  return request.orgContext?.orgId ?? request.agentOrgId ?? null;
}

/** Scan authored values without feeding schema property names to heuristics. */
function authoredText(value: unknown): string {
  if (typeof value === 'string') return value;
  if (Array.isArray(value)) return value.map(authoredText).join('\n');
  if (value && typeof value === 'object') {
    return Object.values(value as Record<string, unknown>).map(authoredText).join('\n');
  }
  return '';
}

function requireWorkspace(
  request: FastifyRequest,
  reply: FastifyReply,
  workspaceRepo: WorkspaceRepo,
  workspaceId: string,
): boolean {
  const organizationId = organizationIdOf(request);
  if (!organizationId || !workspaceRepo.findByIdInOrg(workspaceId, organizationId)) {
    void reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'workspace not found' } });
    return false;
  }
  return true;
}

/** Public API for validating, creating, inspecting, diffing, and publishing specs. */
export function registerAgentSpecificationRoutes(
  app: FastifyInstance,
  deps: { repo: AgentSpecificationRepo; workspaceRepo: WorkspaceRepo; promptScanRepo?: PromptScanRepo },
): void {
  app.post('/api/workspaces/:workspaceId/agent-specifications/validate', async (request, reply) => {
    const params = parseParams(reply, WorkspaceParams, request.params);
    if (!params.ok || !requireWorkspace(request, reply, deps.workspaceRepo, params.data.workspaceId)) return;
    const body = parseBody(reply, ValidateBody, request.body);
    if (!body.ok) return;
    const result = AgentSpecificationSchema.safeParse(body.data.specification);
    if (!result.success) return reply.send({ valid: false, issues: result.error.issues });
    const security = scan({ text: authoredText(result.data) });
    if (security.verdict === 'block') {
      return reply.send({
        valid: false,
        issues: security.findings
          .filter((finding) => finding.severity === 'block')
          .map((finding) => ({ code: finding.rule, message: finding.message, path: [] })),
        security,
      });
    }
    return reply.send({ valid: true, specification: result.data, security });
  });

  app.post('/api/workspaces/:workspaceId/agent-specifications', async (request, reply) => {
    const params = parseParams(reply, WorkspaceParams, request.params);
    if (!params.ok || !requireWorkspace(request, reply, deps.workspaceRepo, params.data.workspaceId)) return;
    const body = parseBody(reply, CreateBody, request.body);
    if (!body.ok) return;
    const specification = AgentSpecificationSchema.safeParse(body.data.specification);
    if (!specification.success) return reply.code(422).send({ error: { code: 'VALIDATION_ERROR', message: 'invalid agent specification', issues: specification.error.issues } });
    const security = scan({ text: authoredText(specification.data) });
    if (security.verdict === 'block') {
      return reply.code(422).send({
        error: {
          code: 'PROMPT_SECURITY_BLOCKED',
          message: 'agent specification contains blocked security findings',
          findings: security.findings,
        },
      });
    }
    const record = await deps.repo.create({
      workspaceId: params.data.workspaceId,
      specification: specification.data,
      author: request.userId ?? 'system',
      changeReason: body.data.changeReason,
      ...(body.data.parentHash === undefined ? {} : { parentHash: body.data.parentHash }),
    });
    if (deps.promptScanRepo) {
      const organizationId = organizationIdOf(request);
      if (organizationId) {
        deps.promptScanRepo.record({
          organizationId,
          actorId: request.userId ?? null,
          resourceKind: 'agent-specification',
          resourceId: record.hash,
          verdict: security.verdict,
          findings: security.findings,
        });
      }
    }
    return reply.code(201).send({ ...record, security });
  });

  app.get('/api/workspaces/:workspaceId/agent-specifications', async (request, reply) => {
    const params = parseParams(reply, WorkspaceParams, request.params);
    if (!params.ok || !requireWorkspace(request, reply, deps.workspaceRepo, params.data.workspaceId)) return;
    const query = parseQuery(reply, ListQuery, request.query);
    if (!query.ok) return;
    return reply.send(deps.repo.list(params.data.workspaceId, query.data));
  });

  app.post('/api/workspaces/:workspaceId/agent-specifications/diff', async (request, reply) => {
    const params = parseParams(reply, WorkspaceParams, request.params);
    if (!params.ok || !requireWorkspace(request, reply, deps.workspaceRepo, params.data.workspaceId)) return;
    const body = parseBody(reply, DiffBody, request.body);
    if (!body.ok) return;
    return reply.send({ changes: await deps.repo.diff(params.data.workspaceId, body.data.leftHash, body.data.rightHash) });
  });

  app.get('/api/workspaces/:workspaceId/agent-specifications/:hash', async (request, reply) => {
    const params = parseParams(reply, HashParams, request.params);
    if (!params.ok || !requireWorkspace(request, reply, deps.workspaceRepo, params.data.workspaceId)) return;
    return reply.send(await deps.repo.get(params.data.workspaceId, params.data.hash));
  });

  app.get('/api/workspaces/:workspaceId/agent-specifications/:hash/lineage', async (request, reply) => {
    const params = parseParams(reply, HashParams, request.params);
    if (!params.ok || !requireWorkspace(request, reply, deps.workspaceRepo, params.data.workspaceId)) return;
    return reply.send({ items: deps.repo.listLineage(params.data.workspaceId, params.data.hash) });
  });

  app.post('/api/workspaces/:workspaceId/agent-specifications/:hash/publish', async (request, reply) => {
    const params = parseParams(reply, HashParams, request.params);
    if (!params.ok || !requireWorkspace(request, reply, deps.workspaceRepo, params.data.workspaceId)) return;
    deps.repo.publish(params.data.workspaceId, params.data.hash);
    return reply.send(await deps.repo.get(params.data.workspaceId, params.data.hash));
  });
}
