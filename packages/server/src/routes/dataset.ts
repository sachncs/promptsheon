import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import {
  CreateDatasetSchema,
  PaginationSchema,
} from '@promptsheon/shared';
import type { DatasetRepo } from '../repos/dataset.js';
import { parseBody, parseQuery } from './validate.js';

const ListQuerySchema = PaginationSchema.extend({
  capabilityId: z.string().uuid().optional(),
});

const CreateCaseSchema = z.object({
  inputs: z.string().min(1),
  expected: z.string().min(1),
  description: z.string().max(2000).optional().default(''),
});

interface RequestOrganizationContext {
  agentOrgId?: string;
  orgContext?: { orgId?: string; organizationId?: string };
}

function requireOrganization(request: unknown, reply: { code: (status: number) => { send: (body: unknown) => unknown } }): string | null {
  const context = (request as RequestOrganizationContext | undefined) ?? {};
  const organizationId = context.orgContext?.orgId ?? context.orgContext?.organizationId ?? context.agentOrgId;
  if (organizationId) return organizationId;
  void reply.code(401).send({ error: { code: 'NO_ORG_CONTEXT', message: 'missing organization context' } });
  return null;
}

export function registerDatasetRoutes(app: FastifyInstance, repo: DatasetRepo) {
  app.get('/api/datasets', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsed = parseQuery(reply, ListQuerySchema, request.query);
    if (!parsed.ok) return;
    const { capabilityId, page, pageSize } = parsed.data;
    if (capabilityId) return reply.send(repo.findByCapabilityIdInOrg(capabilityId, organizationId));
    return reply.send(repo.findManyInOrg(organizationId, { page, pageSize }));
  });

  app.get('/api/datasets/:id', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const { id } = request.params as { id: string };
    const item = repo.findByIdInOrg(id, organizationId);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.post('/api/datasets', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsed = parseBody(reply, CreateDatasetSchema, request.body);
    if (!parsed.ok) return;
    const item = repo.createInOrg(parsed.data, organizationId);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'capability not found' } });
    return reply.code(201).send(item);
  });

  app.delete('/api/datasets/:id', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const { id } = request.params as { id: string };
    repo.deleteInOrg(id, organizationId);
    return reply.code(204).send();
  });

  app.get('/api/datasets/:id/cases', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const { id } = request.params as { id: string };
    const cases = repo.findCasesInOrg(id, organizationId);
    if (!cases) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'dataset not found' } });
    return reply.send(cases);
  });

  app.post('/api/datasets/:id/cases', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, CreateCaseSchema, request.body);
    if (!parsed.ok) return;
    const item = repo.addCaseInOrg(id, organizationId, parsed.data);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'dataset not found' } });
    return reply.code(201).send(item);
  });

  app.delete('/api/datasets/:datasetId/cases/:caseId', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const { datasetId, caseId } = request.params as { datasetId: string; caseId: string };
    repo.deleteCaseInOrg(caseId, datasetId, organizationId);
    return reply.code(204).send();
  });
}
