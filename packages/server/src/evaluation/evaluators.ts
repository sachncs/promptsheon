import { Agent } from '@strands-agents/sdk';
import { z } from 'zod';
import type { AppConfig } from '@promptsheon/shared';
import { createModel } from '../agents/model.js';
import { extractText } from '../agents/utils.js';

/**
 * Common interface for evaluators.
 * Mirrors the future Strands Evals SDK interface.
 */
export interface EvalInput {
  actual: string;
  expected: string;
  inputs: Record<string, unknown>;
  context?: Record<string, unknown>;
}

export interface EvalResult {
  score: number;
  passed: boolean;
  reasoning: string;
}

export interface Evaluator {
  readonly name: string;
  evaluate(input: EvalInput): Promise<EvalResult>;
}

class DeterministicEvaluator implements Evaluator {
  readonly name = 'deterministic';

  async evaluate(input: EvalInput): Promise<EvalResult> {
    const actual = canonicalText(input.actual);
    const expected = canonicalText(input.expected);
    const passed = actual === expected;
    return {
      score: passed ? 1 : 0,
      passed,
      reasoning: passed ? 'actual output matches expected output' : 'actual output does not match expected output',
    };
  }
}

function canonicalText(value: string): string {
  const trimmed = value.trim();
  try {
    return JSON.stringify(JSON.parse(trimmed));
  } catch {
    return trimmed;
  }
}

const NUMERIC_RESULT = z.object({
  score: z.number().min(0).max(1),
  passed: z.boolean(),
  reasoning: z.string(),
});

/**
 * LLM-as-judge evaluator. Generic; can be configured for any dimension
 * by passing a different system prompt at construction time.
 */
class LLMJudgeEvaluator implements Evaluator {
  constructor(public readonly name: string, private config: AppConfig, private systemPrompt: string) {}

  async evaluate(input: EvalInput): Promise<EvalResult> {
    const agent = new Agent({
      model: createModel(this.config),
      systemPrompt: this.systemPrompt,
      structuredOutputSchema: NUMERIC_RESULT,
    });
    const result = await agent.invoke(JSON.stringify(input));
    return NUMERIC_RESULT.parse(JSON.parse(extractText(result)));
  }
}

/**
 * Factory for the bundled evaluators. Adds the 4 we expose to clients.
 */
export function buildEvaluatorRegistry(config: AppConfig): Map<string, Evaluator> {
  const reg = new Map<string, Evaluator>();
  reg.set('deterministic', new DeterministicEvaluator());
  reg.set('llm-judge', new LLMJudgeEvaluator('llm-judge', config,
    'You are an evaluation judge. Score the actual output against expected on 0-1. Return JSON { score, passed, reasoning }.',
  ));
  reg.set('helpfulness', new LLMJudgeEvaluator('helpfulness', config,
    'You are evaluating helpfulness. Score 0-1 how helpful the actual response is. Return JSON { score, passed, reasoning }.',
  ));
  reg.set('coherence', new LLMJudgeEvaluator('coherence', config,
    'You are evaluating coherence and structure. Score 0-1. Return JSON { score, passed, reasoning }.',
  ));
  reg.set('correctness', new LLMJudgeEvaluator('correctness', config,
    'You are evaluating factual correctness. Score 0-1. Return JSON { score, passed, reasoning }.',
  ));
  reg.set('goal-success-rate', new LLMJudgeEvaluator('goal-success-rate', config,
    'You are evaluating whether the actual output achieves the stated goal. Score 0-1. Return JSON { score, passed, reasoning }.',
  ));
  return reg;
}

export function getEvaluator(registry: Map<string, Evaluator>, name: string): Evaluator {
  const e = registry.get(name);
  if (!e) throw new Error(`unknown evaluator: ${name}`);
  return e;
}

export function listEvaluators(registry: Map<string, Evaluator>): string[] {
  return Array.from(registry.keys()).sort();
}
