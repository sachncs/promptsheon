import type { FastifyInstance, FastifyReply, FastifyRequest } from 'fastify';
import { z } from 'zod';
import { AgentSpecificationSchema } from '@promptsheon/shared';
import type { AgentSpecificationRepo } from '../repos/agent-specification.js';
import type { WorkspaceRepo } from '../repos/workspace.js';
import { parseBody, parseParams } from './validate.js';

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

function organizationIdOf(request: FastifyRequest): string | null {
  return request.orgContext?.orgId ?? request.agentOrgId ?? null;
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
  deps: { repo: AgentSpecificationRepo; workspaceRepo: WorkspaceRepo },
): void {
  app.post('/api/workspaces/:workspaceId/agent-specifications/validate', async (request, reply) => {
    const params = parseParams(reply, WorkspaceParams, request.params);
    if (!params.ok || !requireWorkspace(request, reply, deps.workspaceRepo, params.data.workspaceId)) return;
    const body = parseBody(reply, ValidateBody, request.body);
    if (!body.ok) return;
    const result = AgentSpecificationSchema.safeParse(body.data.specification);
    return reply.send(result.success
      ? { valid: true, specification: result.data }
      : { valid: false, issues: result.error.issues });
  });

  app.post('/api/workspaces/:workspaceId/agent-specifications', async (request, reply) => {
    const params = parseParams(reply, WorkspaceParams, request.params);
    if (!params.ok || !requireWorkspace(request, reply, deps.workspaceRepo, params.data.workspaceId)) return;
    const body = parseBody(reply, CreateBody, request.body);
    if (!body.ok) return;
    const specification = AgentSpecificationSchema.safeParse(body.data.specification);
    if (!specification.success) return reply.code(422).send({ error: { code: 'VALIDATION_ERROR', message: 'invalid agent specification', issues: specification.error.issues } });
    const record = await deps.repo.create({
      workspaceId: params.data.workspaceId,
      specification: specification.data,
      author: request.userId ?? 'system',
      changeReason: body.data.changeReason,
      ...(body.data.parentHash === undefined ? {} : { parentHash: body.data.parentHash }),
    });
    return reply.code(201).send(record);
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
