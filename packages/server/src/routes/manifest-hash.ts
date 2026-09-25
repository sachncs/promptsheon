import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { ManifestSchema, mergeDraftManifest } from '@promptsheon/shared';
import type { ManifestRepo } from '../repos/manifest.js';
import { parseBody, parseParams } from './validate.js';
import { NotFoundError } from '@promptsheon/shared';

const ManifestHashParamsSchema = z.object({ hash: z.string().trim().min(1).max(255) });

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
    const manifest = parsed.data;
    const meta = manifest.metadata as Record<string, unknown>;
    const goal = typeof meta['goal'] === 'string' ? meta['goal'] : '';
    const createdBy = typeof meta['createdBy'] === 'string' ? meta['createdBy'] : 'unknown';
    const hash = deps.manifestRepo.create(manifest, { goal, createdBy });
    return reply.code(201).send({ hash });
  });

  app.get('/api/manifests/:hash', async (request, reply) => {
    const parsedParams = parseParams(reply, ManifestHashParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { hash } = parsedParams.data;
    const manifest = deps.manifestRepo.findByHash(hash);
    if (!manifest) throw new NotFoundError('manifest', hash);
    return reply.send(manifest);
  });

  /**
   * Pure-validation endpoint. Accepts a manifest body (possibly
   * incomplete — a draft from the editor), runs the same Zod
   * schema as POST /api/manifests, and returns the parsed issue
   * list. Never persists. Used by local tooling and CI validation.
   */
  app.post('/api/manifests/validate', async (request, reply) => {
    let merged: Record<string, unknown>;
    try {
      merged = mergeDraftManifest(request.body);
    } catch {
      return reply.code(400).send({
        valid: false,
        issues: [{ path: [], message: 'manifest body must be a JSON object' }],
      });
    }
    const parsed = ManifestSchema.safeParse(merged);
    if (!parsed.success) {
      return reply.code(200).send({
        valid: false,
        issues: parsed.error.issues.map((i) => ({
          path: i.path,
          message: i.message,
          code: i.code,
        })),
      });
    }
    return reply.code(200).send({ valid: true, issues: [] });
  });
}
