import type { FastifyInstance } from 'fastify';
import type { VersionRepo } from '../repos/version.js';

export function registerManifestRoutes(app: FastifyInstance, versionRepo: VersionRepo) {
  app.get('/api/manifests/:versionId', async (request, reply) => {
    const { versionId } = request.params as { versionId: string };
    const version = versionRepo.findById(versionId);
    if (!version) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(JSON.parse(version.manifest));
  });
}
