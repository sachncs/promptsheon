import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import type { Agent } from '@strands-agents/sdk';
import { SnapshotStore } from '../snapshots/store.js';
import { parseBody } from './validate.js';

const CreateSnapshotSchema = z.object({
  agentId: z.string().min(1),
});

export function registerSnapshotRoutes(app: FastifyInstance, deps: { store: SnapshotStore; getAgent: (id: string) => Agent | null }) {
  app.post('/api/snapshots', async (request, reply) => {
    const parsed = parseBody(reply, CreateSnapshotSchema, request.body);
    if (!parsed.ok) return;
    const agent = deps.getAgent(parsed.data.agentId);
    if (!agent) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Agent not found' } });
    const { meta, snapshot } = await deps.store.capture(agent);
    return reply.code(201).send({ meta, snapshotPreview: 'redacted' });
  });

  app.get('/api/snapshots', async (_request, reply) => {
    return reply.send({ snapshots: deps.store.list() });
  });

  app.post('/api/snapshots/:id/restore', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, CreateSnapshotSchema, request.body);
    if (!parsed.ok) return;
    const agent = deps.getAgent(parsed.data.agentId);
    if (!agent) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Agent not found' } });
    try {
      await deps.store.restore(agent, id);
      return reply.send({ ok: true, agentId: parsed.data.agentId, snapshotId: id });
    } catch (e) {
      return reply.code(500).send({ error: { code: 'RESTORE_FAILED', message: (e as Error).message } });
    }
  });
}