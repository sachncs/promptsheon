import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { CreateReleaseSchema, PaginationSchema } from '@promptsheon/shared';
import type { ReleaseRepo } from '../repos/release.js';
import { parseBody, parseQuery } from './validate.js';

const ListQuerySchema = PaginationSchema.extend({
  capabilityId: z.string().uuid().optional(),
});

const CreateBodySchema = CreateReleaseSchema.extend({
  capabilityVersionId: z.string().uuid().nullable(),
  manifest: z.string().min(1),
  createdBy: z.string().optional(),
});

const CanaryBodySchema = z.object({
  percent: z.number().int().min(0).max(100),
});

/**
 * Select a release for an invocation using per-request random canary split.
 * Each active release in the (capability, env) pool gets weight = canaryPercent.
 * Falls back to the only active release if there's only one.
 */
export function selectByCanary(
  pool: Array<{ id: string; canaryPercent: number }>,
  rng: () => number = Math.random,
): string | null {
  if (pool.length === 0) return null;
  if (pool.length === 1) return pool[0].id;
  const total = pool.reduce((sum, r) => sum + r.canaryPercent, 0);
  if (total <= 0) return pool[0].id;
  const r = rng() * total;
  let acc = 0;
  for (const release of pool) {
    acc += release.canaryPercent;
    if (r < acc) return release.id;
  }
  return pool[pool.length - 1].id;
}

export function registerReleaseRoutes(app: FastifyInstance, repo: ReleaseRepo) {
  app.get('/api/releases', async (request, reply) => {
    const parsed = parseQuery(reply, ListQuerySchema, request.query);
    if (!parsed.ok) return;
    const { capabilityId, page, pageSize } = parsed.data;
    if (capabilityId) return reply.send(repo.findByCapabilityId(capabilityId));
    return reply.send(repo.findMany({ page, pageSize }));
  });

  app.get('/api/releases/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = repo.findById(id);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.post('/api/releases', async (request, reply) => {
    const parsed = parseBody(reply, CreateBodySchema, request.body);
    if (!parsed.ok) return;
    const item = repo.create(parsed.data);
    return reply.code(201).send(item);
  });

  app.put('/api/releases/:id/activate', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = repo.updateStatus(id, 'active');
    return reply.send(item);
  });

  app.put('/api/releases/:id/supersede', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = repo.updateStatus(id, 'superseded');
    return reply.send(item);
  });

  app.put('/api/releases/:id/canary', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, CanaryBodySchema, request.body);
    if (!parsed.ok) return;
    const item = repo.findById(id);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    const updated = repo.updateCanaryPercent(id, parsed.data.percent);
    return reply.send(updated);
  });
}
