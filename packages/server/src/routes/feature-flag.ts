import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { parseBody } from './validate.js';
import type { FeatureFlagRepo } from '../repos/feature-flag.js';
import type { AuditChain } from '../audit/chain.js';
import { requireAdmin } from '../middleware/admin.js';

const PutFeatureFlagSchema = z.object({
  name: z.string().min(1).max(120).regex(/^[a-z0-9._-]+$/, {
    message: 'name must match /^[a-z0-9._-]+$/',
  }),
  enabled: z.boolean().default(false),
  description: z.string().max(500).optional().default(''),
  value: z.unknown().optional(),
});

const FeatureFlagInputSchema = PutFeatureFlagSchema.omit({ name: true });

interface RequestUserContext {
  userId?: string;
}

function actorOf(request: unknown): string {
  const ctx = (request as RequestUserContext | undefined) ?? {};
  return ctx.userId ?? 'system';
}

export function registerFeatureFlagRoutes(
  app: FastifyInstance,
  deps: { repo: FeatureFlagRepo; auditChain: AuditChain },
) {
  app.get('/api/feature-flags', { preHandler: requireAdmin() }, async (_request, reply) => {
    return reply.send({ flags: deps.repo.findMany() });
  });

  app.put('/api/feature-flags/:name', { preHandler: requireAdmin() }, async (request, reply) => {
    const { name } = request.params as { name: string };
    const parsedInput = parseBody(reply, FeatureFlagInputSchema, request.body);
    if (!parsedInput.ok) return;
    const parsed = PutFeatureFlagSchema.safeParse({ ...parsedInput.data, name });
    if (!parsed.success) {
      return reply.code(422).send({ error: { code: 'VALIDATION_ERROR', message: 'Request body validation failed' } });
    }
    const before = deps.repo.find(name);
    const flag = deps.repo.upsert(parsed.data);
    deps.auditChain.append({
      userId: actorOf(request),
      action: before ? 'feature_flag.update' : 'feature_flag.create',
      resource: 'feature_flag',
      details: JSON.stringify({ name, enabled: parsed.data.enabled }),
      resourceKind: 'feature_flag',
      resourceId: name,
    });
    return reply.send(flag);
  });

  app.delete('/api/feature-flags/:name', { preHandler: requireAdmin() }, async (request, reply) => {
    const { name } = request.params as { name: string };
    const removed = deps.repo.delete(name);
    if (!removed) {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'feature flag not found' } });
    }
    deps.auditChain.append({
      userId: actorOf(request),
      action: 'feature_flag.delete',
      resource: 'feature_flag',
      details: JSON.stringify({ name }),
      resourceKind: 'feature_flag',
      resourceId: name,
    });
    return reply.code(204).send();
  });
}
