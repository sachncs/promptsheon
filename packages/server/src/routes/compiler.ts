import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { ManifestSchema } from '@promptsheon/shared';
import type { ReasoningCompiler } from '../agents/compiler/compiler.js';
import { parseBody } from './validate.js';

const CompileSchema = z.object({
  manifest: ManifestSchema,
  capabilityContext: z.string().optional(),
  constraints: z.array(z.string()).optional(),
});

const DecompileSchema = z.object({
  manifest: ManifestSchema,
});

export function registerCompilerRoutes(app: FastifyInstance, compiler: ReasoningCompiler) {
  app.post('/api/compiler/compile', async (request, reply) => {
    const parsed = parseBody(reply, CompileSchema, request.body);
    if (!parsed.ok) return;
    const { manifest, capabilityContext, constraints } = parsed.data;
    const compiled = await compiler.compile(manifest, { capabilityContext, constraints });
    return reply.send(compiled);
  });

  app.post('/api/compiler/decompile', async (request, reply) => {
    const parsed = parseBody(reply, DecompileSchema, request.body);
    if (!parsed.ok) return;
    const { manifest } = parsed.data;
    const original = await compiler.decompile(manifest);
    return reply.send({ original });
  });
}