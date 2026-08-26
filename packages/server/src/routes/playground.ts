import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type { Gateway, GatewayRequest } from '../llm/gateway.js';
import { parseBody } from './validate.js';

const CompleteSchema = z.object({
  prompt: z.string().min(1).max(64_000),
  model: z.string().min(1).max(120),
  provider: z.enum(['openai', 'anthropic', 'bedrock', 'custom']),
  temperature: z.coerce.number().min(0).max(2).default(0.7),
  baseUrl: z.string().url().optional(),
  apiKey: z.string().min(1).optional(),
  signal: z.unknown().optional(),
});

const SweepVariantSchema = z.object({
  prompt: z.string().min(1).max(64_000),
  temperature: z.coerce.number().min(0).max(2).default(0.7),
});

const SweepSchema = z.object({
  base: z.object({
    prompt: z.string().min(1).max(64_000),
    model: z.string().min(1).max(120),
    provider: z.enum(['openai', 'anthropic', 'bedrock', 'custom']),
    baseUrl: z.string().url().optional(),
    apiKey: z.string().min(1).optional(),
  }),
  variants: z.array(SweepVariantSchema).min(1).max(10),
});

interface RequestUserContext {
  userId?: string;
}

function actorOf(request: unknown): string {
  const ctx = (request as RequestUserContext | undefined) ?? {};
  return ctx.userId ?? 'unscoped';
}

/**
 * Playground routes — non-mutating surface for prompt iteration.
 *   POST /api/playground/complete — fire one prompt against the
 *     gateway; cache hits are returned for free.
 *   POST /api/playground/sweep — fire N variants (different
 *     temperatures or prompt edits) and return side-by-side.
 *
 * Both endpoints run as the calling actor for rate-limit purposes.
 */
export function registerPlaygroundRoutes(app: FastifyInstance, deps: { gateway: Gateway }) {
  app.post('/api/playground/complete', async (request, reply) => {
    const parsed = parseBody(reply, CompleteSchema, request.body);
    if (!parsed.ok) return;
    const data = parsed.data;
    const gwRequest: GatewayRequest = {
      prompt: data.prompt,
      model: data.model,
      provider: data.provider,
      temperature: data.temperature,
    };
    if (data.baseUrl) gwRequest.baseUrl = data.baseUrl;
    if (data.apiKey) gwRequest.apiKey = data.apiKey;
    try {
      const result = await deps.gateway.complete(gwRequest, { actorId: actorOf(request) });
      return reply.send(result);
    } catch (err) {
      const status = (err as { statusCode?: number }).statusCode ?? 502;
      return reply.code(status).send({
        error: {
          code: status === 429 ? 'RATE_LIMITED' : 'PROVIDER_ERROR',
          message: (err as Error).message,
        },
      });
    }
  });

  app.post('/api/playground/sweep', async (request, reply) => {
    const parsed = parseBody(reply, SweepSchema, request.body);
    if (!parsed.ok) return;
    const { base, variants } = parsed.data;
    const actorId = actorOf(request);
    const results = await Promise.allSettled(
      variants.map((v) =>
        deps.gateway.complete(
          {
            prompt: v.prompt,
            model: base.model,
            provider: base.provider,
            temperature: v.temperature,
          },
          { actorId },
        ),
      ),
    );
    return reply.send({
      base: { model: base.model, provider: base.provider },
      variants: results.map((r, i) => ({
        variant: variants[i],
        status: r.status,
        value: r.status === 'fulfilled' ? r.value : undefined,
        error: r.status === 'rejected' ? (r.reason as Error).message : undefined,
      })),
    });
  });
}
