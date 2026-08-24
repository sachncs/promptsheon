import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type { Manifest } from '@promptsheon/shared';
import type { ReasoningCompiler } from '../agents/compiler/compiler.js';
import { parseBody } from './validate.js';

const CompileSchema = z
  .object({
    manifest: z.unknown().optional(),
    capabilityContext: z.string().optional(),
    constraints: z.array(z.string()).optional(),
    prompt: z.string().min(1).optional(),
  })
  .refine((v) => v.manifest !== undefined || v.prompt !== undefined, {
    message: 'either manifest or prompt is required',
  });

const DecompileSchema = z.object({
  manifest: z.unknown(),
});

/**
 * Build a minimal Manifest wrapping a raw prompt string. Used by the
 * /api/compiler/compile route when the body is {prompt: string} —
 * the frontend's legacy contract — so the LLM still receives a
 * structured object to reason over.
 */
function buildManifestFromPrompt(prompt: string): Manifest {
  const now = new Date().toISOString();
  return {
    id: `derived-${Math.random().toString(36).slice(2, 10)}`,
    version: 1,
    prompt: { systemPrompt: prompt },
    model: { provider: 'anthropic', name: 'claude-sonnet', temperature: 0.2, topP: 1, maxTokens: 1024 },
    runtime: { timeoutMs: 30_000, retries: 1 },
    context: { inputs: {}, required: [] },
    memory: { mode: 'none' },
    guardrails: { pre: [], post: [] },
    tools: [],
    mcpServers: [],
    evaluation: { datasetId: null, scorers: [], passThreshold: 0.7, borderlineBand: 0.1 },
    nodes: [],
    edges: [],
    metadata: { source: 'compiler.synthesize' },
    createdAt: now,
    updatedAt: now,
  };
}

export function registerCompilerRoutes(app: FastifyInstance, compiler: ReasoningCompiler) {
  app.post('/api/compiler/compile', async (request, reply) => {
    const parsed = parseBody(reply, CompileSchema, request.body);
    if (!parsed.ok) return;
    const manifest =
      parsed.data.manifest !== undefined
        ? (parsed.data.manifest as Manifest)
        : buildManifestFromPrompt(parsed.data.prompt ?? '');
    const compiled = await compiler.compile(manifest, {
      capabilityContext: parsed.data.capabilityContext,
      constraints: parsed.data.constraints,
    });
    return reply.send(compiled);
  });

  app.post('/api/compiler/decompile', async (request, reply) => {
    const parsed = parseBody(reply, DecompileSchema, request.body);
    if (!parsed.ok) return;
    const { manifest } = parsed.data;
    const original = await compiler.decompile(manifest as Manifest);
    return reply.send({ original });
  });
}