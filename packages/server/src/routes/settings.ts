import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type { SettingsResolver } from '../settings/resolver.js';
import { parseBody } from './validate.js';
import { requireAdmin } from '../middleware/admin.js';

const SetSettingSchema = z.object({
  value: z.unknown(),
});

export function registerSettingsRoutes(app: FastifyInstance, resolver: SettingsResolver) {
  app.get('/api/settings', async (_request, reply) => {
    return reply.send(await resolver.list());
  });

  app.get('/api/settings/:key', async (request, reply) => {
    const { key } = request.params as { key: string };
    const value = await resolver.get(key);
    if (value === undefined) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Setting not found' } });
    return reply.send({ key, value });
  });

  app.put('/api/settings/:key', { preHandler: requireAdmin() }, async (request, reply) => {
    const { key } = request.params as { key: string };
    const parsed = parseBody(reply, SetSettingSchema, request.body);
    if (!parsed.ok) return;
    await resolver.set(key, parsed.data.value);
    return reply.send({ key, value: parsed.data.value });
  });
}