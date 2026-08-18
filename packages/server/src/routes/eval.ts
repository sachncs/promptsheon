import type { FastifyInstance } from 'fastify';
import type { EvalRepo } from '../repos/eval.js';
import type { EvaluationAgent } from '../agents/evaluation/evaluation.js';

export function registerEvalRoutes(app: FastifyInstance, repo: EvalRepo, evalAgent: EvaluationAgent) {
  app.get('/api/eval-runs', async (request, reply) => {
    const { releaseId, page = 1, pageSize = 20 } = request.query as { releaseId?: string; page?: number; pageSize?: number };
    if (releaseId) return reply.send(repo.findRunsByReleaseId(releaseId));
    return reply.send(repo.findMany({ page, pageSize }));
  });

  app.get('/api/eval-runs/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const item = repo.findRunById(id);
    if (!item) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Not found' } });
    return reply.send(item);
  });

  app.post('/api/eval-runs', async (request, reply) => {
    const data = request.body as { releaseId: string; datasetId: string; scorer: string };
    const item = repo.createRun(data);
    return reply.code(201).send(item);
  });

  app.get('/api/eval-runs/:id/results', async (request, reply) => {
    const { id } = request.params as { id: string };
    return reply.send(repo.findResultsByRunId(id));
  });

  app.post('/api/eval/run', async (request, reply) => {
    const { evalRunId, getActualUrl } = request.body as { evalRunId: string; getActualUrl: string };
    const evalRun = repo.findRunById(evalRunId);
    if (!evalRun) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Eval run not found' } });

    const getActual = async (inputs: Record<string, unknown>): Promise<string> => {
      const res = await fetch(getActualUrl, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(inputs),
      });
      return res.text();
    };

    const result = await evalAgent.runEval(evalRun, [], getActual);
    repo.updateRun(evalRunId, result);
    return reply.send(result);
  });
}
