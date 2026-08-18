import type { AppConfig, EvalRun, DatasetCase } from '@promptsheon/shared';
import type { Scorer } from './scorers.js';
import { LLMScorer, ExactMatchScorer, ContainsScorer } from './scorers.js';

export class EvaluationAgent {
  private scorers: Map<string, Scorer>;

  constructor(config: AppConfig) {
    this.scorers = new Map<string, Scorer>([
      ['llm-judge', new LLMScorer(config)],
      ['exact-match', new ExactMatchScorer()],
      ['contains', new ContainsScorer()],
    ]);
  }

  async runEval(
    evalRun: EvalRun,
    cases: DatasetCase[],
    getActual: (inputs: Record<string, unknown>) => Promise<string>,
    onProgress?: (completed: number, total: number) => void,
  ): Promise<EvalRun> {
    const scorer = this.scorers.get(evalRun.scorer) ?? this.scorers.get('llm-judge')!;
    let passed = 0;
    let failed = 0;

    for (let i = 0; i < cases.length; i++) {
      const testCase = cases[i];
      const inputs = JSON.parse(testCase.inputs) as Record<string, unknown>;
      const expected = JSON.parse(testCase.expected);

      const actual = await getActual(inputs);
      const result = await scorer.score({
        actual,
        expected: JSON.stringify(expected),
        inputs,
      });

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
