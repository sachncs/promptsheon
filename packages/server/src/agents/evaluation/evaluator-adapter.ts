import type { AppConfig } from '@promptsheon/shared';
import { EvaluatorRegistry } from './registry.js';
import { EVALUATOR_NAMES, type EvaluatorName } from './registry.js';

export interface AdapterInput {
  actual: string;
  expected?: string;
  inputs?: Record<string, unknown>;
  context?: Record<string, unknown>;
  /** Optional override of the manifest's manifestHash. */
  manifestHash?: string;
}

export interface AdapterResult {
  score: number;
  passed: boolean;
  reasoning: string;
  evaluatorName: string;
}

/**
 * Top-level adapter. Routes a score request through the
 * {@link EvaluatorRegistry} for the named evaluator and normalises the
 * result to a stable `AdapterResult` shape. The class is intentionally
 * lightweight — it exists so that the goal loop and eval suite share
 * a single conversion path (no duplicated normalisation logic).
 *
 * Construction is sync; scoring is async because each evaluator may
 * invoke an LLM.
 */
export class StrandsEvaluatorAdapter {
  private readonly registry: EvaluatorRegistry;

  constructor(private readonly config: AppConfig, private readonly evaluatorName: EvaluatorName | string) {
    if (!EVALUATOR_NAMES.includes(evaluatorName as EvaluatorName)) {
      throw new Error(`unknown evaluator name: ${evaluatorName}`);
    }
    this.registry = new EvaluatorRegistry(config);
  }

  async score(input: AdapterInput): Promise<AdapterResult> {
    const evaluator = this.registry.get(this.evaluatorName);
    const result = await evaluator.evaluate({
      actual: input.actual,
      expected: input.expected ?? '',
      inputs: input.inputs ?? {},
      context: { ...input.context, manifestHash: input.manifestHash },
    });
    return {
      score: result.score,
      passed: result.passed,
      reasoning: result.reasoning,
      evaluatorName: evaluator.name,
    };
  }
}

export { EvaluatorRegistry, EVALUATOR_NAMES };
export type { EvaluatorName };