import type { AppConfig, EvalRun, DatasetCase, Manifest } from '@promptsheon/shared';
import {
  buildEvaluatorRegistry,
  getEvaluator,
  type Evaluator,
  type EvalInput,
  type EvalResult,
} from '../../evaluation/evaluators.js';
import { EvalSuiteRunner } from './suite-runner.js';

export class EvaluationAgent {
  private evaluators: Map<string, Evaluator>;
  private suiteRunner: EvalSuiteRunner;

  constructor(config: AppConfig) {
    this.evaluators = buildEvaluatorRegistry(config);
    this.suiteRunner = new EvalSuiteRunner(config);
  }

  /**
   * Run the eval suite: for each case, get the actual output from
   * getActual(), score it with the configured evaluator, and aggregate.
   *
   * The evalRun.scorer field selects the evaluator. Defaults to
   * 'llm-judge' if missing.
   *
   * When `manifest` is provided and lists multiple scorers in
   * `manifest.evaluation.scorers`, the multi-scorer suite-runner is used
   * instead of the single-scorer path.
   */
  async runEval(
    evalRun: EvalRun,
    cases: DatasetCase[],
    getActual: (inputs: Record<string, unknown>) => Promise<string>,
    onProgress?: (completed: number, total: number) => void,
    manifest?: Manifest,
  ): Promise<EvalRun> {
    const multiScorer = manifest && manifest.evaluation.scorers.length > 1;
    if (multiScorer) {
      return this.runMultiScorerEval(evalRun, cases, getActual, onProgress, manifest);
    }

    const scorerName = evalRun.scorer || 'llm-judge';
    const evaluator = getEvaluator(this.evaluators, scorerName);

    let passed = 0;
    let failed = 0;

    for (let i = 0; i < cases.length; i++) {
      const testCase = cases[i];
      const inputs = JSON.parse(testCase.inputs) as Record<string, unknown>;
      const expected = JSON.parse(testCase.expected);

      const actual = await getActual(inputs);
      const evalInput: EvalInput = {
        actual,
        expected: JSON.stringify(expected),
        inputs,
      };
      const result: EvalResult = await evaluator.evaluate(evalInput);

      if (result.passed) passed++;
      else failed++;

      onProgress?.(i + 1, cases.length);
    }

    return {
      ...evalRun,
      score: cases.length === 0 ? 0 : passed / cases.length,
      passed,
      failed,
      total: cases.length,
      status: failed === 0 && cases.length > 0 ? 'passed' : 'failed',
      finishedAt: new Date().toISOString(),
    };
  }

  private async runMultiScorerEval(
    evalRun: EvalRun,
    cases: DatasetCase[],
    getActual: (inputs: Record<string, unknown>) => Promise<string>,
    onProgress: ((completed: number, total: number) => void) | undefined,
    manifest: Manifest,
  ): Promise<EvalRun> {
    let casesPassed = 0;
    let casesFailed = 0;

    for (let i = 0; i < cases.length; i++) {
      const testCase = cases[i];
      const inputs = JSON.parse(testCase.inputs) as Record<string, unknown>;
      const expected = JSON.parse(testCase.expected);
      const actual = await getActual(inputs);

      const suiteResult = await this.suiteRunner.run(manifest, {
        actual,
        expected: JSON.stringify(expected),
        inputs,
      });

      if (suiteResult.passed) casesPassed++;
      else casesFailed++;

      onProgress?.(i + 1, cases.length);
    }

    return {
      ...evalRun,
      score: cases.length === 0 ? 0 : casesPassed / cases.length,
      passed: casesPassed,
      failed: casesFailed,
      total: cases.length,
      status: casesFailed === 0 && cases.length > 0 ? 'passed' : 'failed',
      finishedAt: new Date().toISOString(),
    };
  }
}