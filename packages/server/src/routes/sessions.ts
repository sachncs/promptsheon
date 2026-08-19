import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { SessionStore } from '../sessions/store.js';
import { parseBody } from './validate.js';

const CreateSessionSchema = z.object({
  capabilityVersionId: z.string().optional(),
});

const AppendSchema = z.object({
  messages: z.array(z.object({
    role: z.enum(['user', 'assistant']),
    content: z.array(z.union([
      z.object({ text: z.string() }),
      z.object({ type: z.string(), text: z.string().optional() }),
    ])),
  })),
});

export function registerSessionRoutes(app: FastifyInstance, deps: { store: SessionStore }) {
  app.post('/api/sessions', async (request, reply) => {
    parseBody(reply, CreateSessionSchema, request.body);
    const session = await deps.store.create();
    return reply.code(201).send(session);
  });

  app.get('/api/sessions/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const session = deps.store.get(id);
    if (!session) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Session not found' } });
    return reply.send(session);
  });

  app.post('/api/sessions/:id/messages', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, AppendSchema, request.body);
    if (!parsed.ok) return;
    const session = await deps.store.appendMessages(id, parsed.data.messages as never);
    if (!session) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Session not found' } });
    return reply.send(session);
  });

  app.delete('/api/sessions/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const ok = await deps.store.delete(id);
    if (!ok) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Session not found' } });
    return reply.code(204).send();
  });
}