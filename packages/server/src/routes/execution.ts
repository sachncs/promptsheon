import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import { InvokeExecutionSchema } from '@promptsheon/shared';
import type { ExecutionRepo } from '../repos/execution.js';
import type { InvocationAgent } from '../agents/invocation.js';
import { parseBody, parseQuery } from './validate.js';

const ListExecutionsQuerySchema = z.object({
  capabilityVersionId: z.string().min(1).optional(),
  page: z.number().int().min(1).default(1),
  pageSize: z.number().int().min(1).max(100).default(20),
});

export function registerExecutionRoutes(app: FastifyInstance, repo: ExecutionRepo, invocationAgent: InvocationAgent) {
  app.get('/api/executions', async (request, reply) => {
    const parsed = parseQuery(reply, ListExecutionsQuerySchema, request.query);
    if (!parsed.ok) return;
    const { capabilityVersionId, page, pageSize } = parsed.data;
    if (capabilityVersionId) return reply.send(repo.findByVersionId(capabilityVersionId, { page, pageSize }));
    return reply.send(repo.findMany({ page, pageSize }));
  });

  app.get('/api/executions/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = repo.findById(id);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.post('/api/invoke', async (request, reply) => {
    const parsed = parseBody(reply, InvokeExecutionSchema, request.body);
    if (!parsed.ok) return;
    const { capabilityVersionId, inputs, environment, traceId } = parsed.data;
    const execution = await invocationAgent.invoke(capabilityVersionId, inputs, { environment, traceId });
    repo.create({
      capabilityVersionId,
      inputs: JSON.stringify(inputs),
      outputs: execution.outputs,
      model: execution.model,
      provider: execution.provider,
      latencyMs: execution.latencyMs,
      costUsd: execution.costUsd,
      promptTokens: execution.promptTokens,
      completionTokens: execution.completionTokens,
      totalTokens: execution.totalTokens,
      error: execution.error,
      traceId: execution.traceId,
      environment: execution.environment,
    });
    return reply.send(execution);
  });
}