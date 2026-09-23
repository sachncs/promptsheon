import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import {
  CreateEvalRunSchema,
  PaginationSchema,
} from '@promptsheon/shared';
import type { EvalRepo } from '../repos/eval.js';
import type { EvaluationAgent } from '../agents/evaluation/evaluation.js';
import { buildEvaluatorRegistry, listEvaluators } from '../evaluation/evaluators.js';
import { parseBody, parseQuery } from './validate.js';
import { validateOutboundUrl } from '../security/outbound-url.js';

const ListQuerySchema = PaginationSchema.extend({
  releaseId: z.string().uuid().optional(),
});

const RunEvalSchema = z.object({
  evalRunId: z.string().uuid(),
  getActualUrl: z.string().url(),
});

const ScoreInputSchema = z.object({
  actual: z.string(),
  expected: z.string(),
  inputs: z.record(z.string(), z.unknown()).default({}),
  context: z.record(z.string(), z.unknown()).optional(),
  evaluator: z.string().optional(),
});

export function registerEvalRoutes(app: FastifyInstance, repo: EvalRepo, evalAgent: EvaluationAgent) {
  app.get('/api/eval-runs', async (request, reply) => {
    const parsed = parseQuery(reply, ListQuerySchema, request.query);
    if (!parsed.ok) return;
    const { releaseId, page, pageSize } = parsed.data;
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
    const parsed = parseBody(reply, CreateEvalRunSchema, request.body);
    if (!parsed.ok) return;
    const item = repo.createRun(parsed.data);
    return reply.code(201).send(item);
  });

  app.get('/api/eval-runs/:id/results', async (request, reply) => {
    const { id } = request.params as { id: string };
    return reply.send(repo.findResultsByRunId(id));
  });

  app.post('/api/eval/run', async (request, reply) => {
    const parsed = parseBody(reply, RunEvalSchema, request.body);
    if (!parsed.ok) return;
    const { evalRunId, getActualUrl } = parsed.data;
    const outbound = validateOutboundUrl(getActualUrl, {
      allowedHosts: (process.env['PROMPTSHEON_EVAL_ALLOWED_HOSTS'] ?? '').split(',').map((host) => host.trim()),
      allowPrivateNetworks: (process.env['PROMPTSHEON_NODE_ENV'] ?? process.env['NODE_ENV'] ?? 'development') !== 'production',
    });
    if (!outbound.ok) {
      return reply.code(422).send({ error: { code: 'UNSAFE_OUTBOUND_URL', message: outbound.reason } });
    }
    const evalRun = repo.findRunById(evalRunId);
    if (!evalRun) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'Eval run not found' } });

    const getActual = async (inputs: Record<string, unknown>): Promise<string> => {
      const controller = new AbortController();
      const timeout = setTimeout(() => controller.abort(), 15_000);
      let res: Response;
      try {
        res = await fetch(outbound.url, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(inputs),
          signal: controller.signal,
        });
      } finally {
        clearTimeout(timeout);
      }
      if (!res.ok) throw new Error(`actual endpoint returned ${res.status}`);
      const body = await res.arrayBuffer();
      if (body.byteLength > 2 * 1024 * 1024) throw new Error('actual endpoint response exceeds 2 MiB');
      return new TextDecoder().decode(body);
    };

    let result;
    try {
      result = await evalAgent.runEval(evalRun, [], getActual);
    } catch {
      return reply.code(502).send({ error: { code: 'EVAL_ENDPOINT_FAILED', message: 'evaluation endpoint failed' } });
    }
    repo.updateRun(evalRunId, result);
    return reply.send(result);
  });

  app.get('/api/eval/evaluators', async (_request, reply) => {
    const config = (evalAgent as unknown as { config: import('@promptsheon/shared').AppConfig }).config;
    const reg = buildEvaluatorRegistry(config);
    return reply.send({ evaluators: listEvaluators(reg) });
  });

  app.post('/api/eval/score', async (request, reply) => {
    const parsed = parseBody(reply, ScoreInputSchema, request.body);
    if (!parsed.ok) return;
    const config = (evalAgent as unknown as { config: import('@promptsheon/shared').AppConfig }).config;
    const reg = buildEvaluatorRegistry(config);
    const evaluatorName = parsed.data.evaluator || 'llm-judge';
    const evaluator = reg.get(evaluatorName);
    if (!evaluator) {
      return reply.code(404).send({ error: { code: 'UNKNOWN_EVALUATOR', message: evaluatorName } });
    }
    const result = await evaluator.evaluate({
      actual: parsed.data.actual,
      expected: parsed.data.expected,
      inputs: parsed.data.inputs,
      context: parsed.data.context,
    });
    return reply.send({
      evaluator: evaluatorName,
      ...result,
    });
  });
}
