import { randomUUID } from 'node:crypto';
import { passAtK, type EvalSuite, type EvalSuiteVersion, type GraderSpec } from '@promptsheon/shared';

/** A single externally supplied trial evaluated by a suite. */
export interface EvalTrial {
  caseId: string;
  output: string;
  transcript?: string;
  finalState?: Record<string, unknown>;
  toolCalls?: Array<{ tool: string; args: Record<string, unknown>; result?: unknown }>;
  referenceTranscript?: string;
}

/** Input accepted by a suite run. */
export interface EvalSuiteRunInput {
  suiteVersionId?: string;
  n?: number;
  k?: number;
  trials?: EvalTrial[];
}

/** Minimal grader result required by the suite use cases. */
export interface GraderResult {
  results: unknown[];
  weightedScore: number;
  passed: boolean;
}

/** Port for deterministic or provider-backed grader implementations. */
export interface GraderFactory {
  create(specs: GraderSpec[]): { run(input: EvalTrial): GraderResult };
}

/** Persistence port for tenant-aware suite and version lookup. */
export interface EvalSuiteStore {
  findById(id: string): EvalSuite | null;
  findByIdInOrg(id: string, organizationId: string): EvalSuite | null;
  findVersion(id: string, version: number): EvalSuiteVersion | null;
  findVersionInOrg(id: string, version: number, organizationId: string): EvalSuiteVersion | null;
  findVersionById(id: string): EvalSuiteVersion | null;
  findVersionByIdInOrg(id: string, organizationId: string): EvalSuiteVersion | null;
  list(): EvalSuite[];
  listForRepositoryInOrg(repositoryId: string, organizationId: string): EvalSuite[];
}

/** Port for placing borderline trials into human review. */
export interface HumanReviewQueue {
  enqueue(caseId: string, suiteId: string, suiteRunId: string | null): unknown;
}

/** Durable persistence port for suite runs and per-trial grader results. */
export interface EvalSuiteRunStore {
  createRun(input: { id: string; suiteId: string; suiteVersionId: string; n: number; k: number; startedAt: string }): void;
  addTrialResult(input: {
    runId: string;
    seq: number;
    caseId: string;
    passed: boolean;
    weightedScore: number;
    trial: Record<string, unknown>;
    graderResult: Record<string, unknown>;
  }): void;
  finishRun(input: { id: string; passAtK: number; rawScore: number; passed: boolean; borderlineCount: number; finishedAt: string }): void;
  failRun(id: string, error: string, finishedAt?: string): void;
}

export type EvalSuiteRunResult =
  | { kind: 'suite-not-found' }
  | { kind: 'version-not-found' }
  | {
      kind: 'success';
      value: {
        runId: string;
        suiteId: string;
        suiteVersionId: string;
        passThreshold: number;
        passAtK: number;
        rawScore: number;
        passed: boolean;
        borderlineCount: number;
        gradedAt: string;
        results: Array<{ trial: EvalTrial; result: GraderResult }>;
      };
    };

export interface EvalGateSummary {
  ok: boolean;
  score: number;
  regressions: Array<{ suiteId: string; suiteName: string; ok: boolean; rawScore: number; threshold: number }>;
  suites: Array<{ suiteId: string; suiteName: string; ok: boolean; rawScore: number; threshold: number }>;
}

function serializeTrial(trial: EvalTrial): Record<string, unknown> {
  return {
    caseId: trial.caseId,
    output: trial.output,
    ...(trial.transcript === undefined ? {} : { transcript: trial.transcript }),
    ...(trial.finalState === undefined ? {} : { finalState: trial.finalState }),
    ...(trial.toolCalls === undefined ? {} : { toolCalls: trial.toolCalls }),
    ...(trial.referenceTranscript === undefined ? {} : { referenceTranscript: trial.referenceTranscript }),
  };
}

function serializeGraderResult(result: GraderResult): Record<string, unknown> {
  return {
    results: result.results,
    weightedScore: result.weightedScore,
    passed: result.passed,
  };
}

/** Coordinates suite version selection, grading, scoring, and review routing. */
export class EvalSuiteService {
  constructor(
    private readonly store: EvalSuiteStore,
    private readonly graders: GraderFactory,
    private readonly reviews: HumanReviewQueue,
    private readonly runStore?: EvalSuiteRunStore,
  ) {}

  run(
    suiteId: string,
    organizationId: string | undefined,
    input: EvalSuiteRunInput,
  ): EvalSuiteRunResult {
    const suite = organizationId
      ? this.store.findByIdInOrg(suiteId, organizationId)
      : this.store.findById(suiteId);
    if (!suite) return { kind: 'suite-not-found' };

    const version = input.suiteVersionId
      ? organizationId
        ? this.store.findVersionByIdInOrg(input.suiteVersionId, organizationId)
        : this.store.findVersionById(input.suiteVersionId)
      : organizationId
        ? this.store.findVersionInOrg(suiteId, suite.currentVersion, organizationId)
        : this.store.findVersion(suiteId, suite.currentVersion);
    if (!version) return { kind: 'version-not-found' };

    const trials = input.trials ?? [{ caseId: 'sample-1', output: 'hello', finalState: {} }];
    const n = input.n ?? trials.length;
    const k = input.k ?? 1;
    const runner = this.graders.create(version.graderConfig);
    const runId = `run-${randomUUID()}`;
    this.runStore?.createRun({
      id: runId,
      suiteId: suite.id,
      suiteVersionId: version.id,
      n,
      k,
      startedAt: new Date().toISOString(),
    });
    let results: Array<{ trial: EvalTrial; result: GraderResult }>;
    try {
      results = trials.map((trial) => ({ trial, result: runner.run(trial) }));
    } catch (error) {
      this.runStore?.failRun(runId, error instanceof Error ? error.message : 'grader failed');
      throw error;
    }
    const successes = results.filter(({ result }) => result.passed).length;
    const rawScore = results.reduce((total, { result }) => total + result.weightedScore, 0) / Math.max(1, results.length);
    const borderline = results.filter(
      ({ result }) => Math.abs(result.weightedScore - suite.passThreshold) <= suite.borderlineBand && !result.passed,
    );
    results.forEach(({ trial, result }, seq) => {
      this.runStore?.addTrialResult({
        runId,
        seq,
        caseId: trial.caseId,
        passed: result.passed,
        weightedScore: result.weightedScore,
        trial: serializeTrial(trial),
        graderResult: serializeGraderResult(result),
      });
    });
    const passAtKValue = passAtK(n, k, successes);
    const passed = rawScore >= suite.passThreshold;
    this.runStore?.finishRun({
      id: runId,
      passAtK: passAtKValue,
      rawScore,
      passed,
      borderlineCount: borderline.length,
      finishedAt: new Date().toISOString(),
    });
    for (const { trial, result } of results) {
      if (Math.abs(result.weightedScore - suite.passThreshold) <= suite.borderlineBand) {
        this.reviews.enqueue(trial.caseId, suite.id, runId);
      }
    }

    return {
      kind: 'success',
      value: {
        runId,
        suiteId: suite.id,
        suiteVersionId: version.id,
        passThreshold: suite.passThreshold,
        passAtK: passAtKValue,
        rawScore,
        passed,
        borderlineCount: borderline.length,
        gradedAt: new Date().toISOString(),
        results,
      },
    };
  }

  gate(repositoryId: string, organizationId: string | undefined, trials: EvalTrial[]): EvalGateSummary {
    const suites = organizationId
      ? this.store.listForRepositoryInOrg(repositoryId, organizationId)
      : this.store.list();
    const summaries = suites.flatMap((suite) => {
      const version = organizationId
        ? this.store.findVersionInOrg(suite.id, suite.currentVersion, organizationId)
        : this.store.findVersion(suite.id, suite.currentVersion);
      if (!version) return [];
      const runner = this.graders.create(version.graderConfig);
      const graded = trials.map((trial) => runner.run(trial));
      const rawScore = graded.reduce((total, result) => total + result.weightedScore, 0) / Math.max(1, graded.length);
      return [{ suiteId: suite.id, suiteName: suite.name, ok: rawScore >= suite.passThreshold, rawScore, threshold: suite.passThreshold }];
    });
    return {
      ok: summaries.every((summary) => summary.ok),
      score: summaries[0]?.rawScore ?? 1,
      regressions: summaries.filter((summary) => !summary.ok),
      suites: summaries,
    };
  }
}
