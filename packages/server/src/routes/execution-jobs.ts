import type { FastifyInstance, FastifyReply, FastifyRequest } from 'fastify';
import { z } from 'zod';
import { DurableExecutionService, ExecutionWorkspaceScopeError } from '../application/durable-execution-service.js';
import type { WorkspaceRepo } from '../repos/workspace.js';
import { IdempotencyConflictError } from '../repos/execution-job.js';
import { parseBody, parseParams } from './validate.js';

const CreateJobSchema = z.strictObject({
  agentHash: z.string().regex(/^[0-9a-f]{64}$/),
  inputs: z.record(z.string(), z.unknown()).refine((value) => JSON.stringify(value).length <= 1_000_000, 'inputs exceed 1 MB'),
  idempotencyKey: z.string().trim().min(1).max(255),
  maxAttempts: z.number().int().min(1).max(10).optional(),
});
const JobParamsSchema = z.strictObject({ id: z.string().uuid() });
const WorkspaceParamsSchema = z.strictObject({ workspaceId: z.string().uuid() });
const WorkspaceJobParamsSchema = WorkspaceParamsSchema.extend(JobParamsSchema.shape);

function organizationIdOf(request: FastifyRequest): string | null {
  return request.orgContext?.orgId ?? request.agentOrgId ?? null;
}

function requireOrganization(request: FastifyRequest, reply: FastifyReply): string | null {
  const organizationId = organizationIdOf(request);
  if (organizationId) return organizationId;
  void reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
  return null;
}

/** HTTP adapter for durable, asynchronous execution jobs. */
export function registerExecutionJobRoutes(app: FastifyInstance, deps: { service: DurableExecutionService; workspaceRepo: WorkspaceRepo }): void {
  app.post('/api/workspaces/:workspaceId/execution-jobs', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const params = parseParams(reply, WorkspaceParamsSchema, request.params);
    if (!params.ok || !deps.workspaceRepo.findByIdInOrg(params.data.workspaceId, organizationId)) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'workspace not found' } });
    const parsed = parseBody(reply, CreateJobSchema, request.body);
    if (!parsed.ok) return;
    let job;
    try {
      job = deps.service.enqueue({ organizationId, workspaceId: params.data.workspaceId, ...parsed.data });
    } catch (error) {
      if (error instanceof IdempotencyConflictError) return reply.code(409).send({ error: { code: 'IDEMPOTENCY_CONFLICT', message: error.message } });
      throw error;
    }
    return reply.code(202).send(job);
  });

  app.get('/api/execution-jobs/metrics', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    return reply.send(deps.service.metrics(organizationId));
  });

  app.get('/api/workspaces/:workspaceId/execution-jobs/:id', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsed = parseParams(reply, WorkspaceJobParamsSchema, request.params);
    if (!parsed.ok) return;
    try {
      return reply.send(deps.service.get(organizationId, parsed.data.workspaceId, parsed.data.id));
    } catch (error) {
      if (error instanceof ExecutionWorkspaceScopeError) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'execution job not found' } });
      throw error;
    }
  });

  app.post('/api/workspaces/:workspaceId/execution-jobs/:id/cancel', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsed = parseParams(reply, WorkspaceJobParamsSchema, request.params);
    if (!parsed.ok) return;
    try {
      return reply.send(deps.service.cancel(organizationId, parsed.data.workspaceId, parsed.data.id));
    } catch (error) {
      if (error instanceof ExecutionWorkspaceScopeError) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'execution job not found' } });
      throw error;
    }
  });
}
