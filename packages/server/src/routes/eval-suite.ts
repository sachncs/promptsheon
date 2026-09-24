import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import {
  cohensKappa,
  krippendorffAlpha,
  type EvalSuite,
  type GraderSpec,
} from '@promptsheon/shared';
import {
  type EvalSuiteRepo,
  type HumanReviewRepo,
} from '../repos/eval-suite.js';
import type { EvalSuiteService } from '../application/eval-suite-service.js';
import { parseBody, parseQuery } from './validate.js';
import { registerRouteDoc } from '../openapi.js';

function actorOf(request: unknown): string {
  const ctx = (request as { userId?: string } | undefined) ?? {};
  return ctx.userId ?? 'system';
}

function organizationIdOf(request: { orgContext?: { orgId?: string }; agentOrgId?: string }): string | undefined {
  return request.orgContext?.orgId ?? request.agentOrgId;
}

const CreateSuiteSchema = z.object({
  capabilityId: z.string(),
  repositoryId: z.string().nullable().optional(),
  name: z.string().min(1).max(120),
  description: z.string().max(500).optional(),
  passThreshold: z.number().min(0).max(1).optional(),
  borderlineBand: z.number().min(0).max(1).optional(),
  notes: z.string().optional(),
  initialGraders: z
    .array(
      z.object({
        name: z.string().min(1),
        kind: z.enum([
          'regex_match',
          'schema_state_check',
          'tool_call_assertion',
          'transcript_diff',
          'llm_rubric',
        ]),
        weight: z.number().min(0).max(1),
        config: z.record(z.string(), z.unknown()),
      }),
    )
    .optional(),
});

const RunSuiteSchema = z.object({
  suiteVersionId: z.string().optional(),
  releaseId: z.string().optional(),
  n: z.number().int().min(1).max(20).optional(),
  k: z.number().int().min(1).max(10).optional(),
  trials: z
    .array(
      z.object({
        caseId: z.string(),
        output: z.string(),
        transcript: z.string().optional(),
        finalState: z.record(z.string(), z.unknown()).optional(),
        toolCalls: z
          .array(
            z.object({
              tool: z.string(),
              args: z.record(z.string(), z.unknown()),
              result: z.unknown().optional(),
            }),
          )
          .optional(),
        referenceTranscript: z.string().optional(),
      }),
    )
    .optional(),
});

const GateSchema = z.object({
  trials: z
    .array(
      z.object({
        caseId: z.string(),
        output: z.string(),
        transcript: z.string().optional(),
        finalState: z.record(z.string(), z.unknown()).optional(),
        toolCalls: z
          .array(
            z.object({
              tool: z.string(),
              args: z.record(z.string(), z.unknown()),
              result: z.unknown().optional(),
            }),
          )
          .optional(),
      }),
    )
    .min(1),
});

const ReviewDecisionSchema = z.object({
  decision: z.enum(['approve', 'reject']),
  notes: z.string().max(2000).optional(),
});

const ListSuiteQuerySchema = z.object({
  capabilityId: z.string().min(1).max(200).optional(),
});

const CalibrationSchema = z.object({
  a: z.array(z.string().max(100_000)).min(1).max(10_000),
  b: z.array(z.string().max(100_000)).min(1).max(10_000),
}).superRefine((value, context) => {
  if (value.a.length !== value.b.length) {
    context.addIssue({
      code: z.ZodIssueCode.custom,
      message: 'equal non-empty arrays required',
      path: ['b'],
    });
  }
});

export interface EvalSuiteRouteDeps {
  suiteRepo: EvalSuiteRepo;
  humanReviewRepo: HumanReviewRepo;
  suiteExecution?: EvalSuiteService;
}

export function registerEvalSuiteRoutes(
  app: FastifyInstance,
  deps: EvalSuiteRouteDeps,
): void {
  app.get('/api/eval-suites', async (request, reply) => {
    const parsed = parseQuery(reply, ListSuiteQuerySchema, request.query);
    if (!parsed.ok) return;
    const { capabilityId } = parsed.data;
    const organizationId = organizationIdOf(request);
    return reply.send(organizationId ? deps.suiteRepo.listInOrg(organizationId, capabilityId) : deps.suiteRepo.list(capabilityId));
  });
  registerRouteDoc({
    method: 'get',
    path: '/api/eval-suites',
    summary: 'List eval suites (?capabilityId=... to filter)',
    tags: ['evals'],
  });

  app.post('/api/eval-suites', async (request, reply) => {
    const parsed = parseBody(reply, CreateSuiteSchema, request.body);
    if (!parsed.ok) return;
    const initial: GraderSpec[] = (parsed.data.initialGraders ?? []).map((g) => ({
      name: g.name,
      kind: g.kind,
      weight: g.weight,
      // The Zod record coerce widens the config to `Record<string, unknown>`
      // but our grader spec expects a discriminator-bearing union; cast at
      // the boundary. The runner validates `kind` again at run time.
      config: g.config as never,
    }));
    const input = {
      capabilityId: parsed.data.capabilityId,
      repositoryId: parsed.data.repositoryId ?? null,
      name: parsed.data.name,
      description: parsed.data.description ?? null,
      passThreshold: parsed.data.passThreshold ?? 0.92,
      borderlineBand: parsed.data.borderlineBand ?? 0.05,
      createdBy: actorOf(request),
      initialGraders: initial,
      notes: parsed.data.notes ?? null,
    };
    const organizationId = organizationIdOf(request);
    const out = organizationId ? deps.suiteRepo.createInOrg(input, organizationId) : deps.suiteRepo.create(input);
    if (!out) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'capability not found' } });
    return reply.code(201).send(out);
  });
  registerRouteDoc({
    method: 'post',
    path: '/api/eval-suites',
    summary: 'Create an eval suite with version-1 grader config',
    tags: ['evals'],
    body: CreateSuiteSchema,
  });

  app.get('/api/eval-suites/:id', async (request, reply) => {
    const { id } = request.params as { id: string };
    const organizationId = organizationIdOf(request);
    const suite = organizationId ? deps.suiteRepo.findByIdInOrg(id, organizationId) : deps.suiteRepo.findById(id);
    if (!suite) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'suite not found' } });
    return reply.send({ suite, versions: deps.suiteRepo.listVersions(id) });
  });
  registerRouteDoc({
    method: 'get',
    path: '/api/eval-suites/:id',
    summary: 'Fetch a suite and its version history',
    tags: ['evals'],
  });

  app.post('/api/eval-suites/:id/run', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, RunSuiteSchema, request.body ?? {});
    if (!parsed.ok) return;
    const organizationId = organizationIdOf(request);
    const result = deps.suiteExecution?.run(id, organizationId, parsed.data);
    if (!result || result.kind === 'suite-not-found') {
      return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'suite not found' } });
    }
    if (result.kind === 'version-not-found') {
      return reply.code(404).send({ error: { code: 'NOT_VERSION', message: 'suite has no versioned graders' } });
    }
    return reply.code(201).send(result.value);
  });

  /**
   * CI gate — standalone endpoint usable from any external CI.
   * Accepts a list of graded trials and returns pass/fail.
   */
  app.post('/api/repos/:id/eval-gate', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, GateSchema, request.body);
    if (!parsed.ok) return;
    const organizationId = organizationIdOf(request);
    const result = deps.suiteExecution?.gate(id, organizationId, parsed.data.trials);
    if (!result || result.suites.length === 0) {
      return reply.send({
        ok: true,
        score: 1,
        regressions: [],
        suites: [],
        note: `repository ${id} has no suites; gate passes by default`,
      });
    }
    return reply.send(result);
  });

  app.get('/api/human-review', async (request, reply) => {
    const organizationId = organizationIdOf(request);
    return reply.send(organizationId ? deps.humanReviewRepo.listOpenInOrg(organizationId) : deps.humanReviewRepo.listOpen());
  });

  app.post('/api/human-review/:id/decide', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, ReviewDecisionSchema, request.body);
    if (!parsed.ok) return;
    const organizationId = organizationIdOf(request);
    const review = organizationId
      ? deps.humanReviewRepo.decideInOrg(id, organizationId, actorOf(request), parsed.data.decision, parsed.data.notes ?? null)
      : deps.humanReviewRepo.decide(id, actorOf(request), parsed.data.decision, parsed.data.notes ?? null);
    if (!review) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'review not found' } });
    return reply.send(review);
  });

  // Calibration endpoint — accepts two parallel label arrays of
  // equal length, returns Cohen's kappa and Krippendorff's alpha
  // (nominal). Used by /app/eval/calibrations UI.
  app.post('/api/eval/calibrate', async (request, reply) => {
    const parsed = parseBody(reply, CalibrationSchema, request.body);
    if (!parsed.ok) return;
    const { a, b } = parsed.data;
    return reply.send({
      n: a.length,
      cohensKappa: cohensKappa(a, b),
      krippendorffAlpha: krippendorffAlpha(a, b),
    });
  });
}
