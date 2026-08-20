import type { FastifyInstance } from 'fastify';
import { z } from 'zod';
import {
  passAtK,
  type EvalSuite,
  type GraderSpec,
} from '@promptsheon/shared';
import {
  type EvalSuiteRepo,
  type HumanReviewRepo,
} from '../repos/eval-suite.js';
import { GraderRunner } from '../agents/evaluation/grader-runner.js';
import { parseBody } from './validate.js';
import { registerRouteDoc } from '../openapi.js';

function actorOf(request: unknown): string {
  const ctx = (request as { userId?: string } | undefined) ?? {};
  return ctx.userId ?? 'system';
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

export interface EvalSuiteRouteDeps {
  suiteRepo: EvalSuiteRepo;
  humanReviewRepo: HumanReviewRepo;
}

interface RunSummary {
  runId: string;
  suiteId: string;
  suiteVersionId: string;
  passThreshold: number;
  passAtK: number;
  rawScore: number;
  passed: boolean;
  borderlineCount: number;
  gradedAt: string;
}

/**
 * Per-process cache for in-flight runs so the gate endpoint can
 * reuse the latest score without a database roundtrip.
 */
const runCache = new Map<string, RunSummary>();

export function registerEvalSuiteRoutes(
  app: FastifyInstance,
  deps: EvalSuiteRouteDeps,
): void {
  app.get('/api/eval-suites', async (request, reply) => {
    const { capabilityId } = request.query as { capabilityId?: string };
    return reply.send(deps.suiteRepo.list(capabilityId));
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
    const out = deps.suiteRepo.create({
      capabilityId: parsed.data.capabilityId,
      repositoryId: parsed.data.repositoryId ?? null,
      name: parsed.data.name,
      description: parsed.data.description ?? null,
      passThreshold: parsed.data.passThreshold ?? 0.92,
      borderlineBand: parsed.data.borderlineBand ?? 0.05,
      createdBy: actorOf(request),
      initialGraders: initial,
      notes: parsed.data.notes ?? null,
    });
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
    const suite = deps.suiteRepo.findById(id);
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
    const suite = deps.suiteRepo.findById(id);
    if (!suite) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'suite not found' } });
    const version = parsed.data.suiteVersionId
      ? deps.suiteRepo.findVersionById(parsed.data.suiteVersionId)
      : deps.suiteRepo.findVersion(id, suite.currentVersion);
    if (!version) return reply.code(404).send({ error: { code: 'NOT_VERSION', message: 'suite has no versioned graders' } });
    const trials = parsed.data.trials ?? [
      { caseId: 'sample-1', output: 'hello', finalState: {} },
    ];
    const n = parsed.data.n ?? trials.length;
    const k = parsed.data.k ?? 1;
    const runner = new GraderRunner(version.graderConfig);
    const graded = trials.map((t) => ({
      trial: t,
      result: runner.run({
        output: t.output,
        transcript: t.transcript ?? '',
        finalState: t.finalState ?? {},
        toolCalls: t.toolCalls ?? [],
        referenceTranscript: t.referenceTranscript ?? '',
      }),
    }));
    const successes = graded.filter((g) => g.result.passed).length;
    const passAtKValue = passAtK(n, k, successes);
    const rawScore =
      graded.reduce((acc, g) => acc + g.result.weightedScore, 0) / Math.max(1, graded.length);
    const passed = rawScore >= suite.passThreshold;
    const borderlineBand = graded.filter(
      (g) => Math.abs(g.result.weightedScore - suite.passThreshold) <= suite.borderlineBand && !g.result.passed,
    ).length;

    for (const g of graded) {
      if (Math.abs(g.result.weightedScore - suite.passThreshold) <= suite.borderlineBand) {
        deps.humanReviewRepo.enqueue(g.trial.caseId, suite.id, null);
      }
    }

    const runId = `run-${Date.now()}-${Math.floor(Math.random() * 1e6)}`;
    const summary: RunSummary = {
      runId,
      suiteId: suite.id,
      suiteVersionId: version.id,
      passThreshold: suite.passThreshold,
      passAtK: passAtKValue,
      rawScore,
      passed,
      borderlineCount: borderlineBand,
      gradedAt: new Date().toISOString(),
    };
    runCache.set(runId, summary);
    return reply.code(201).send({ ...summary, results: graded });
  });

  /**
   * CI gate — standalone endpoint usable from any external CI.
   * Accepts a list of graded trials and returns pass/fail.
   */
  app.post('/api/repos/:id/eval-gate', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, GateSchema, request.body);
    if (!parsed.ok) return;
    const suites = deps.suiteRepo.list();
    if (suites.length === 0) {
      return reply.send({
        ok: true,
        score: 1,
        regressions: [],
        suites: [],
        note: `repository ${id} has no suites; gate passes by default`,
      });
    }
    // Use each suite's current version + the first graders we have.
    const summaries: Array<{
      suiteId: string;
      suiteName: string;
      ok: boolean;
      rawScore: number;
      threshold: number;
    }> = [];
    for (const suite of suites) {
      const version = deps.suiteRepo.findVersion(suite.id, suite.currentVersion);
      if (!version) continue;
      const runner = new GraderRunner(version.graderConfig);
      const graded = parsed.data.trials.map((t) =>
        runner.run({
          output: t.output,
          transcript: t.transcript ?? '',
          finalState: t.finalState ?? {},
          toolCalls: t.toolCalls ?? [],
          referenceTranscript: '',
        }),
      );
      const rawScore =
        graded.reduce((acc, g) => acc + g.weightedScore, 0) / Math.max(1, graded.length);
      const ok = rawScore >= suite.passThreshold;
      summaries.push({
        suiteId: suite.id,
        suiteName: suite.name,
        ok,
        rawScore,
        threshold: suite.passThreshold,
      });
    }
    const ok = summaries.every((s) => s.ok);
    return reply.send({ ok, score: summaries[0]?.rawScore ?? 1, regressions: summaries.filter((s) => !s.ok), suites: summaries });
  });

  app.get('/api/human-review', async (request, reply) => {
    return reply.send(deps.humanReviewRepo.listOpen());
  });

  app.post('/api/human-review/:id/decide', async (request, reply) => {
    const { id } = request.params as { id: string };
    const parsed = parseBody(reply, ReviewDecisionSchema, request.body);
    if (!parsed.ok) return;
    const review = deps.humanReviewRepo.decide(id, actorOf(request), parsed.data.decision, parsed.data.notes ?? null);
    if (!review) return reply.code(404).send({ error: { code: 'NOT_FOUND', message: 'review not found' } });
    return reply.send(review);
  });

  // Calibration endpoint — accepts two parallel label arrays of
  // equal length, returns Cohen's kappa and Krippendorff's alpha
  // (nominal). Used by /app/eval/calibrations UI.
  app.post('/api/eval/calibrate', async (request, reply) => {
    const body = request.body as { a?: string[]; b?: string[] };
    const a = body.a ?? [];
    const b = body.b ?? [];
    if (a.length !== b.length || a.length === 0) {
      return reply.code(422).send({ error: { code: 'BAD_INPUT', message: 'equal non-empty arrays required' } });
    }
    const { cohensKappa, krippendorffAlpha } = await import('@promptsheon/shared');
    return reply.send({
      n: a.length,
      cohensKappa: cohensKappa(a, b),
      krippendorffAlpha: krippendorffAlpha(a, b),
    });
  });
}
