import type { AppConfig, EvalRun, DatasetCase } from '@promptsheon/shared';
import { LLMScorer, type ScorerResult, type ScorerInput } from './scorers.js';

export class EvaluationAgent {
  private scorer: LLMScorer;

  constructor(config: AppConfig) {
    this.scorer = new LLMScorer(config);
  }

  async runEval(
    evalRun: EvalRun,
    cases: DatasetCase[],
    getActual: (inputs: Record<string, unknown>) => Promise<string>,
    onProgress?: (completed: number, total: number) => void,
  ): Promise<EvalRun> {
    let passed = 0;
    let failed = 0;

    for (let i = 0; i < cases.length; i++) {
      const testCase = cases[i];
      const inputs = JSON.parse(testCase.inputs) as Record<string, unknown>;
      const expected = JSON.parse(testCase.expected);

      const actual = await getActual(inputs);
      const result: ScorerResult = await this.scorer.score({
        actual,
        expected: JSON.stringify(expected),
        inputs,
      } satisfies ScorerInput);

      if (result.passed) passed++;
      else failed++;

      onProgress?.(i + 1, cases.length);
    }

    return {
      ...evalRun,
      score: passed / cases.length,
      passed,
      failed,
      total: cases.length,
      status: failed === 0 ? 'passed' : 'failed',
      finishedAt: new Date().toISOString(),
    };
  }
}