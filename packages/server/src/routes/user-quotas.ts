import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type { UserQuotaRepo } from '../repos/user-quota.js';
import { parseBody, parseParams, parseQuery } from './validate.js';
import { NotFoundError } from '@promptsheon/shared';

const OrganizationQuerySchema = z.object({ organizationId: z.string().trim().min(1).max(200) });
const CreateQuotaSchema = z.object({
  organizationId: z.string().trim().min(1).max(200),
  userId: z.string().trim().min(1).max(200),
  label: z.string().trim().min(1).max(120),
  dailyRuns: z.number().int().min(1).nullable().optional(),
  dailyTokens: z.number().int().min(1).nullable().optional(),
  dailyCostMicros: z.number().int().min(1).nullable().optional(),
  enabled: z.boolean().optional().default(true),
});
const UpdateQuotaSchema = CreateQuotaSchema.omit({ organizationId: true, userId: true }).partial();
const QuotaParamsSchema = z.object({ id: z.string().uuid() });

function activeOrg(request: { orgContext?: { orgId?: string }; agentOrgId?: string }): string | undefined {
  return request.orgContext?.orgId ?? request.agentOrgId;
}

function assertOrgScope(request: { orgContext?: { orgId?: string }; agentOrgId?: string }, organizationId: string, reply: { code: (status: number) => { send: (body: unknown) => unknown } }): boolean {
  const current = activeOrg(request);
  if (current && current !== organizationId) {
    void reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'organization not found' } });
    return false;
  }
  return true;
}

/** Admin CRUD for durable per-user execution quota policies. */
export function registerUserQuotaRoutes(app: FastifyInstance, deps: { quotaRepo: UserQuotaRepo }): void {
  app.get('/api/admin/user-quotas', async (request, reply) => {
    const parsed = parseQuery(reply, OrganizationQuerySchema, request.query);
    if (!parsed.ok) return;
    if (!assertOrgScope(request, parsed.data.organizationId, reply)) return;
    return reply.send({ items: deps.quotaRepo.listForOrg(parsed.data.organizationId) });
  });

  app.post('/api/admin/user-quotas', async (request, reply) => {
    const parsed = parseBody(reply, CreateQuotaSchema, request.body);
    if (!parsed.ok) return;
    if (!assertOrgScope(request, parsed.data.organizationId, reply)) return;
    try {
      return reply.code(201).send(deps.quotaRepo.create(parsed.data));
    } catch (error) {
      if (String(error).includes('UNIQUE')) return reply.code(409).send({ error: { code: 'QUOTA_DUPLICATE', message: 'a quota already exists for this user' } });
      throw error;
    }
  });

  app.patch('/api/admin/user-quotas/:id', async (request, reply) => {
    const params = parseParams(reply, QuotaParamsSchema, request.params);
    if (!params.ok) return;
    const body = parseBody(reply, UpdateQuotaSchema, request.body);
    if (!body.ok) return;
    const existing = deps.quotaRepo.findById(params.data.id);
    if (!existing) throw new NotFoundError('user quota', params.data.id);
    if (!assertOrgScope(request, existing.organizationId, reply)) return;
    return reply.send(deps.quotaRepo.update(params.data.id, body.data));
  });

  app.delete('/api/admin/user-quotas/:id', async (request, reply) => {
    const params = parseParams(reply, QuotaParamsSchema, request.params);
    if (!params.ok) return;
    const existing = deps.quotaRepo.findById(params.data.id);
    if (!existing) throw new NotFoundError('user quota', params.data.id);
    if (!assertOrgScope(request, existing.organizationId, reply)) return;
    if (!deps.quotaRepo.delete(params.data.id)) throw new NotFoundError('user quota', params.data.id);
    return reply.code(204).send();
  });
}
