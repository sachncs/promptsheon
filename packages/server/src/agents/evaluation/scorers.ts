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
    let parsed: unknown;
    try {
      parsed = JSON.parse(extractText(result));
    } catch (error) {
      throw new Error('evaluation scorer returned invalid JSON', { cause: error });
    }
    return ScorerResultSchema.parse(parsed);
  }
}
