import type { FastifyInstance } from 'fastify';
import { createHash } from 'node:crypto';
import { z } from 'zod';
import { PaginationSchema } from '@promptsheon/shared';
import type { VersionRepo } from '../repos/version.js';
import type { ManifestRepo } from '../repos/manifest.js';
import { parseBody, parseParams, parseQuery } from './validate.js';

const ListQuerySchema = PaginationSchema.extend({
  capabilityId: z.string().uuid().optional(),
});

const CreateVersionSchema = z.object({
  capabilityId: z.string().uuid(),
  version: z.number().int().positive(),
  manifest: z.string().min(1),
  manifestHash: z.string().min(1),
  createdBy: z.string().optional(),
  goal: z.string().optional(),
});
const VersionParamsSchema = z.object({ id: z.string().trim().min(1).max(255) });
const VersionManifestParamsSchema = z.object({ versionId: z.string().trim().min(1).max(255) });

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

export function registerVersionRoutes(
  app: FastifyInstance,
  repo: VersionRepo,
  manifestRepo: ManifestRepo,
) {
  app.get('/api/capability-versions', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsed = parseQuery(reply, ListQuerySchema, request.query);
    if (!parsed.ok) return;
    const { capabilityId, page, pageSize } = parsed.data;
    if (capabilityId) return reply.send(repo.findByCapabilityIdInOrg(capabilityId, organizationId));
    return reply.send(repo.findManyInOrg(organizationId, { page, pageSize }));
  });

  app.get('/api/capability-versions/:id', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsedParams = parseParams(reply, VersionParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const item = repo.findByIdInOrg(id, organizationId);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.get('/api/capability-versions/:versionId/manifest', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsedParams = parseParams(reply, VersionManifestParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { versionId } = parsedParams.data;
    const row = repo.findByIdInOrg(versionId, organizationId);
    if (!row) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'version not found' } });
    let parsed: unknown;
    try {
      parsed = JSON.parse(row.manifest);
    } catch {
      parsed = null;
    }
    return reply.send({
      id: row.id,
      hash: row.manifestHash,
      manifest: parsed,
      capabilityId: row.capabilityId,
      capabilityVersion: row.version,
      createdAt: row.createdAt,
      createdBy: row.createdBy,
      size: row.manifest.length,
      approvals: [],
    });
  });

  app.post('/api/capability-versions', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsed = parseBody(reply, CreateVersionSchema, request.body);
    if (!parsed.ok) return;
    const item = repo.createInOrg(parsed.data, organizationId);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'capability not found' } });

    // BUG-1 fix: also register the manifest in manifest_dag so the
    // maker-checker / approval flow can look it up by hash. Without
    // this, no release in the system can ever pass the gate.
    //
    // The release activation gate computes the manifest hash by
    // sha256-hashing the raw manifest string. We do the same here
    // so the registered row matches what the gate will look up.
    try {
      const canonicalHash = createHash('sha256').update(parsed.data.manifest).digest('hex');
      manifestRepo.registerFromRaw({
        capabilityId: parsed.data.capabilityId,
        version: parsed.data.version,
        manifestHash: canonicalHash,
        manifestJson: parsed.data.manifest,
        goal: parsed.data.goal,
        createdBy: parsed.data.createdBy,
      });
    } catch (err) {
      app.log.error({ err }, 'manifest_dag upsert failed (non-fatal)');
    }

    return reply.code(201).send(item);
  });

  app.delete('/api/capability-versions/:id', async (request, reply) => {
    const organizationId = requireOrganization(request, reply);
    if (!organizationId) return;
    const parsedParams = parseParams(reply, VersionParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    repo.deleteInOrg(id, organizationId);
    return reply.code(204).send();
  });
}
