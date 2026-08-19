import type { AppConfig, Manifest } from '@promptsheon/shared';
import { EvaluatorRegistry, type EvaluatorName } from './registry.js';
import { EVALUATOR_NAMES } from './registry.js';

export interface SuiteScorerInput {
  /** Output from the manifest execution. Concatenated node outputs. */
  actual: string;
  /** Per-scorer expected value (goal text, expected answer, etc). */
  expected?: string;
  /** Per-scorer inputs map. */
  inputs?: Record<string, unknown>;
  /** Per-scorer invocation context. */
  context?: Record<string, unknown>;
}

export interface SuiteScorerResult {
  evaluatorName: string;
  score: number;
  passed: boolean;
  reasoning: string;
}

export interface SuiteRunResult {
  scorerResults: SuiteScorerResult[];
  aggregateScore: number;
  passed: boolean;
}

export interface SuiteRunOptions {
  /** Scorer names to run. Falls back to manifest.evaluation.scorers. */
  scorers?: string[];
  /** Override expected/inputs/context for all scorers uniformly. */
  input?: SuiteScorerInput;
  /** Override inputs schema output for `manifest.id`. */
  manifestHash?: string;
}

/**
 * EvalSuiteRunner — runs a list of scorers against a single execution
 * output and aggregates results.
 *
 * Aggregation rules (v1, equal weights):
 * - `aggregateScore = average of all scorer scores`
 * - `passed = all scorers must pass (AND semantics)`
 *
 * The runner is stateless; instances are cheap to create. The
 * {@link EvaluatorRegistry} handles per-evaluator caching.
 */
export class EvalSuiteRunner {
  constructor(private readonly config: AppConfig, private readonly registry: EvaluatorRegistry = new EvaluatorRegistry(config)) {}

  /**
   * Run all scorers in `manifest.evaluation.scorers` (or `options.scorers`)
   * against `actual` output.
   */
  async run(manifest: Manifest, input: SuiteScorerInput, options: SuiteRunOptions = {}): Promise<SuiteRunResult> {
    const declaredScorers = options.scorers ?? manifest.evaluation.scorers;
    const scorerNames = declaredScorers.filter((s): s is EvaluatorName => EVALUATOR_NAMES.includes(s as EvaluatorName));

    if (scorerNames.length === 0) {
      return { scorerResults: [], aggregateScore: 0, passed: false };
    }

    const scorerResults: SuiteScorerResult[] = [];
    for (const name of scorerNames) {
      const evaluator = this.registry.get(name);
      const result = await evaluator.evaluate({
        actual: input.actual,
        expected: input.expected ?? '',
        inputs: input.inputs ?? {},
        context: { ...input.context, manifestHash: options.manifestHash ?? manifest.id },
      });
      scorerResults.push({
        evaluatorName: evaluator.name,
        score: result.score,
        passed: result.passed,
        reasoning: result.reasoning,
      });
    }

    return this.aggregate(scorerResults);
  }

  /**
   * Aggregate a list of pre-computed scorer results using equal-weight
   * averaging and AND-semantics for pass/fail.
   */
  aggregate(scorerResults: SuiteScorerResult[]): SuiteRunResult {
    if (scorerResults.length === 0) {
      return { scorerResults, aggregateScore: 0, passed: false };
    }
    const aggregateScore = scorerResults.reduce((s, r) => s + r.score, 0) / scorerResults.length;
    const passed = scorerResults.every((r) => r.passed);
    return { scorerResults, aggregateScore, passed };
  }
}