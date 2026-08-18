import type { FastifyInstance } from 'fastify';
import type { ExecutionRepo } from '../repos/execution.js';
import type { InvocationAgent } from '../agents/invocation.js';

export function registerExecutionRoutes(app: FastifyInstance, repo: ExecutionRepo, invocationAgent: InvocationAgent) {
  app.get('/api/executions', async (request, reply) => {
    const { capabilityVersionId, page = 1, pageSize = 20 } = request.query as {
      capabilityVersionId?: string; page?: number; pageSize?: number;
    };
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
    const { capabilityVersionId, inputs, environment, traceId } = request.body as {
      capabilityVersionId: string;
      inputs: Record<string, unknown>;
      environment?: string;
      traceId?: string;
    };
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
