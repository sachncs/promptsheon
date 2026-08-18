import type { FastifyInstance } from 'fastify';
import type { SettingsResolver } from '../settings/resolver.js';

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

  app.put('/api/settings/:key', async (request, reply) => {
    const { key } = request.params as { key: string };
    const { value } = request.body as { value: unknown };
    await resolver.set(key, value);
    return reply.send({ key, value });
  });
}
