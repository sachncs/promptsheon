import type { AppConfig, EvalRun, DatasetCase } from '@promptsheon/shared';
import {
  buildEvaluatorRegistry,
  getEvaluator,
  type Evaluator,
  type EvalInput,
  type EvalResult,
} from '../../evaluation/evaluators.js';

export class EvaluationAgent {
  private evaluators: Map<string, Evaluator>;

  constructor(config: AppConfig) {
    this.evaluators = buildEvaluatorRegistry(config);
  }

  /**
   * Run the eval suite: for each case, get the actual output from
   * getActual(), score it with the configured evaluator, and aggregate.
   *
   * The evalRun.scorer field selects the evaluator. Defaults to
   * 'llm-judge' if missing.
   */
  async runEval(
    evalRun: EvalRun,
    cases: DatasetCase[],
    getActual: (inputs: Record<string, unknown>) => Promise<string>,
    onProgress?: (completed: number, total: number) => void,
  ): Promise<EvalRun> {
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
}