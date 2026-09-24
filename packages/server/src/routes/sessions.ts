import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { SessionStore } from '../sessions/store.js';
import { parseBody, parseParams } from './validate.js';

const CreateSessionSchema = z.object({}).strict();
const SessionParamsSchema = z.object({ id: z.string().uuid() });

const AppendSchema = z.object({
  messages: z.array(z.object({
    role: z.enum(['user', 'assistant']),
    content: z.array(z.object({ text: z.string() })),
  })),
});

export function registerSessionRoutes(app: FastifyInstance, deps: { store: SessionStore }) {
  app.post('/api/sessions', async (request, reply) => {
    const parsed = parseBody(reply, CreateSessionSchema, request.body);
    if (!parsed.ok) return;
    const session = await deps.store.create();
    return reply.code(201).send(session);
  });

  app.get('/api/sessions/:id', async (request, reply) => {
    const parsedParams = parseParams(reply, SessionParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const session = deps.store.get(id);
    if (!session) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Session not found' } });
    return reply.send(session);
  });

  app.post('/api/sessions/:id/messages', async (request, reply) => {
    const parsedParams = parseParams(reply, SessionParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const parsed = parseBody(reply, AppendSchema, request.body);
    if (!parsed.ok) return;
    const session = await deps.store.appendMessages(id, parsed.data.messages);
    if (!session) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Session not found' } });
    return reply.send(session);
  });

  app.delete('/api/sessions/:id', async (request, reply) => {
    const parsedParams = parseParams(reply, SessionParamsSchema, request.params);
    if (!parsedParams.ok) return;
    const { id } = parsedParams.data;
    const ok = await deps.store.delete(id);
    if (!ok) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Session not found' } });
    return reply.code(204).send();
  });
}
