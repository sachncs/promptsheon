import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { parseBody } from './validate.js';
import type { IncidentRepo } from '../repos/incident.js';

const ProposeSchema = z.object({
  suiteId: z.string(),
  caseId: z.string().min(1).max(200),
  sourceKind: z.enum(['execution_failure', 'manual']).default('manual'),
  sourceRef: z.string().optional(),
  inputText: z.string().min(1).max(20_000),
  expectedText: z.string().min(1).max(20_000),
  severity: z.enum(['low', 'medium', 'high']).default('medium'),
  notes: z.string().max(2000).optional(),
});

const DecideSchema = z.object({
  decision: z.enum(['accept', 'reject']),
  notes: z.string().max(2000).optional(),
});

export interface IncidentDeps {
  incidentRepo: IncidentRepo;
  actorId: () => string;
}

export function registerIncidentRoutes(app: FastifyInstance, deps: IncidentDeps): void {
  app.get('/api/incidents', async (request, reply) => {
    const { suiteId, status } = request.query as { suiteId?: string; status?: 'open' | 'accepted' | 'rejected' };
    return reply.send(deps.incidentRepo.list({ suiteId, status }));
  });

  app.post('/api/incidents/propose', async (request, reply) => {
    const parsed = parseBody(reply, ProposeSchema, request.body);
    if (!parsed.ok) return;
    const prop = deps.incidentRepo.create({
      suiteId: parsed.data.suiteId,
      caseId: parsed.data.caseId,
      sourceKind: parsed.data.sourceKind,
      sourceRef: parsed.data.sourceRef ?? null,
      inputText: parsed.data.inputText,
      expectedText: parsed.data.expectedText,
      severity: parsed.data.severity,
      proposedBy: deps.actorId(),
      notes: parsed.data.notes,
    });
    return reply.code(201).send(prop);
  });

  app.post('/api/incidents/:id/decide', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, DecideSchema, request.body);
    if (!parsed.ok) return;
    const next = parsed.data.decision === 'accept' ? 'accepted' : 'rejected';
    const updated = deps.incidentRepo.decide(id, deps.actorId(), next, parsed.data.notes ?? null);
    if (!updated) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'incident not found' } });
    return reply.send(updated);
  });
}
