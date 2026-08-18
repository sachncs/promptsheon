import type { AppConfig } from '@promptsheon/shared';
import { Agent } from '@strands-agents/sdk';
import { createModel } from '../model.js';
import { extractText } from '../utils.js';

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

export interface Scorer {
  score(input: ScorerInput): Promise<ScorerResult> | ScorerResult;
}

export class LLMScorer implements Scorer {
  private agent: Agent;

  constructor(config: AppConfig) {
    this.agent = new Agent({
      model: createModel(config),
      systemPrompt: `You are an evaluation scorer. Compare actual outputs against expected outputs and score them.

Score 1.0 = perfect match
Score 0.0 = completely wrong
Score 0.5 = partially correct

Return JSON: { "score": number, "passed": boolean, "reasoning": string }`,
    });
  }

  async score(input: ScorerInput): Promise<ScorerResult> {
    const result = await this.agent.invoke(JSON.stringify(input));
    return JSON.parse(extractText(result));
  }
}

export class ExactMatchScorer implements Scorer {
  score(input: ScorerInput): ScorerResult {
    const passed = input.actual.trim() === input.expected.trim();
    return {
      score: passed ? 1.0 : 0.0,
      passed,
      reasoning: passed ? 'Exact match' : 'Outputs differ',
    };
  }
}

export class ContainsScorer implements Scorer {
  score(input: ScorerInput): ScorerResult {
    const passed = input.actual.includes(input.expected);
    return {
      score: passed ? 1.0 : 0.0,
      passed,
      reasoning: passed ? 'Expected content found' : 'Expected content not found',
    };
  }
}
