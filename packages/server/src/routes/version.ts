import type { FastifyInstance } from 'fastify';
import { createHash } from 'node:crypto';
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
  db?: import('better-sqlite3').Database,
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

  app.get('/api/capability-versions/:versionId/manifest', async (request, reply) => {
    const { versionId } = request.params as { versionId: string };
    const sqlite = (db ?? (app as unknown as { db?: import('better-sqlite3').Database }).db) as
      | import('better-sqlite3').Database
      | undefined;
    if (!sqlite) {
      return reply.code(500).send({ error: { code: 'INTERNAL', message: 'db not configured' } });
    }
    const row = sqlite
      .prepare(
        `SELECT id, capability_id, version, manifest, manifest_hash, created_by, created_at
         FROM capability_versions WHERE id = ?`,
      )
      .get(versionId) as
      | {
          id: string;
          capability_id: string;
          version: number;
          manifest: string;
          manifest_hash: string;
          created_by: string;
          created_at: string;
        }
      | undefined;
    if (!row) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'version not found' } });
    let parsed: unknown;
    try {
      parsed = JSON.parse(row.manifest);
    } catch {
      parsed = null;
    }
    return reply.send({
      id: row.id,
      hash: row.manifest_hash,
      manifest: parsed,
      capabilityId: row.capability_id,
      capabilityVersion: row.version,
      createdAt: row.created_at,
      createdBy: row.created_by,
      size: row.manifest.length,
      approvals: [],
    });
  });

  app.post('/api/capability-versions', async (request, reply) => {
    const parsed = parseBody(reply, CreateVersionSchema, request.body);
    if (!parsed.ok) return;
    const item = repo.create(parsed.data);

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
    const { id } = request.params as { id: string };
    repo.delete(id);
    return reply.code(204).send();
  });
}
