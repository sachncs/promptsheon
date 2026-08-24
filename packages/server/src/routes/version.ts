import { createHash } from 'node:crypto';
import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { PaginationSchema } from '@promptsheon/shared';
import type { VersionRepo } from '../repos/version.js';
import type { ManifestRepo } from '../repos/manifest.js';
import { parseBody, parseQuery } from './validate.js';

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

export function registerVersionRoutes(
  app: FastifyInstance,
  repo: VersionRepo,
  manifestRepo: ManifestRepo,
) {
  app.get('/api/capability-versions', async (request, reply) => {
    const parsed = parseQuery(reply, ListQuerySchema, request.query);
    if (!parsed.ok) return;
    const { capabilityId, page, pageSize } = parsed.data;
    if (capabilityId) return reply.send(repo.findByCapabilityId(capabilityId));
    return reply.send(repo.findMany({ page, pageSize }));
  });

  app.get('/api/capability-versions/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = repo.findById(id);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.post('/api/capability-versions', async (request, reply) => {
    const parsed = parseBody(reply, CreateVersionSchema, request.body);
    if (!parsed.ok) return;
    const item = repo.create(parsed.data);

    // BUG-1 fix: also register the manifest in manifest_dag so the
    // maker-checker / approval flow can look it up by hash. Without
    // this, no release in the system can ever pass the gate.
    //
    // The release activation gate derives the hash from
    // release.manifest using key-sorted JSON canonicalization, so we
    // compute the same hash here and store under it (rather than
    // trusting the client's manifestHash, which may use a different
    // hashing algorithm).
    try {
      const parsedManifest = JSON.parse(parsed.data.manifest) as Record<string, unknown>;
      const canonical = JSON.stringify(parsedManifest, Object.keys(parsedManifest).sort());
      const canonicalHash = createHash('sha256').update(canonical).digest('hex');
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
    const { id } = request.params as { id: string };
    repo.delete(id);
    return reply.code(204).send();
  });
}
