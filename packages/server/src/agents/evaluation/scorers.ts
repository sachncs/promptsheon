import type { AppConfig } from '@promptsheon/shared';
import { Agent } from '@strands-agents/sdk';
import { z } from 'zod';
import { createModel } from '../model.js';
import { extractText } from '../utils.js';

const ScorerResultSchema = z.object({
  score: z.number().min(0).max(1),
  passed: z.boolean(),
  reasoning: z.string(),
});

export interface ScorerInput {
  actual: string;
  expected: string;
  inputs: Record<string, unknown>;
}

export interface ScorerResult {
  score: number;
  passed: boolean;
  reasoning: string;
}

export class LLMScorer {
  private readonly agent?: Agent;
  private readonly simulated: boolean;

  constructor(config: AppConfig) {
    this.simulated = config.llm.defaultProvider === 'simulated';
    if (!this.simulated) {
      this.agent = new Agent({
        model: createModel(config),
        systemPrompt: `You are an evaluation scorer. Compare actual outputs against expected outputs and score them.

Score 1.0 = perfect match
Score 0.0 = completely wrong
Score 0.5 = partially correct

Return JSON: { "score": number, "passed": boolean, "reasoning": string }`,
      });
    }
  }

  async score(input: ScorerInput): Promise<ScorerResult> {
    if (this.simulated) {
      return simulatedScore(input);
    }

    const result = await this.agent!.invoke(JSON.stringify(input));
    let parsed: unknown;
    try {
      parsed = JSON.parse(extractText(result));
    } catch (error) {
      throw new Error('evaluation scorer returned invalid JSON', { cause: error });
    }
    return ScorerResultSchema.parse(parsed);
  }
}

function simulatedScore(input: ScorerInput): ScorerResult {
  const actual = input.actual.trim();
  const expected = input.expected.trim();
  if (actual.length === 0) {
    return { score: 0, passed: false, reasoning: 'simulator: output is empty' };
  }

  if (expected.length > 0 && canonicalText(actual) === canonicalText(expected)) {
    return { score: 1, passed: true, reasoning: 'simulator: output matches the expected value' };
  }

  const score = Math.min(0.9, 0.55 + Math.min(actual.length, 350) / 3500);
  return {
    score,
    passed: score >= 0.5,
    reasoning: 'simulator: deterministic score based on non-empty output shape; semantic scoring requires a configured provider',
  };
}

function canonicalText(value: string): string {
  try {
    return JSON.stringify(JSON.parse(value));
  } catch {
    return value;
  }
}
