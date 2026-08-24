import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { ManifestSchema, mergeDraftManifest } from '@promptsheon/shared';
import type { ManifestRepo } from '../repos/manifest.js';
import { parseBody } from './validate.js';
import { NotFoundError } from '@promptsheon/shared';

export function registerManifestHashRoutes(app: FastifyInstance, deps: { manifestRepo: ManifestRepo }) {
  app.post('/api/manifests', async (request, reply) => {
    let merged: Record<string, unknown>;
    try {
      merged = mergeDraftManifest(request.body);
    } catch {
      return reply.code(400).send({ error: { code: 'INVALID_BODY', message: 'manifest body must be a JSON object' } });
    }
    const parsed = ManifestSchema.safeParse(merged);
    if (!parsed.success) {
      return reply.code(422).send({
        error: {
          code: 'VALIDATION_ERROR',
          message: 'Request validation failed',
          issues: parsed.error.issues,
        },
      });
    }
    const manifest = parsed.data as unknown as Parameters<typeof deps.manifestRepo.create>[0];
    const meta = manifest.metadata as Record<string, unknown>;
    const goal = typeof meta['goal'] === 'string' ? meta['goal'] : '';
    const createdBy = typeof meta['createdBy'] === 'string' ? meta['createdBy'] : 'unknown';
    const hash = deps.manifestRepo.create(manifest, { goal, createdBy });
    return reply.code(201).send({ hash });
  });

  app.get('/api/manifests/:hash', async (request, reply) => {
    const { hash } = request.params as { hash: string };
    const manifest = deps.manifestRepo.findByHash(hash);
    if (!manifest) throw new NotFoundError('manifest', hash);
    return reply.send(manifest);
  });
}