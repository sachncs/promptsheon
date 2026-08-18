import type { FastifyInstance } from 'fastify';
import type { ReasoningCompiler } from '../agents/compiler/compiler.js';
import type { Manifest } from '@promptsheon/shared';

export function registerCompilerRoutes(app: FastifyInstance, compiler: ReasoningCompiler) {
  app.post('/api/compiler/compile', async (request, reply) => {
    const { manifest, capabilityContext, constraints } = request.body as {
      manifest: Manifest;
      capabilityContext?: string;
      constraints?: string[];
    };
    const compiled = await compiler.compile(manifest, { capabilityContext, constraints });
    return reply.send(compiled);
  });

  app.post('/api/compiler/decompile', async (request, reply) => {
    const { manifest } = request.body as { manifest: Manifest };
    const original = await compiler.decompile(manifest);
    return reply.send({ original });
  });
}
