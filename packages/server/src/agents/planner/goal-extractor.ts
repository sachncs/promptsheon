import { Agent } from '@strands-agents/sdk';
import { z } from 'zod';
import type { AppConfig } from '@promptsheon/shared';
import { createModel } from '../model.js';
import { extractText } from '../utils.js';
import { GoalSpecSchema, type GoalSpec } from './types.js';

/**
 * GoalExtractor agent: takes a candidate goal + sub-ideas, refines into
 * a measurable goal with explicit acceptance criteria.
 */
export class GoalExtractorAgent {
  private agent: Agent;

  constructor(config: AppConfig) {
    this.agent = new Agent({
      id: 'goalExtractor',
      model: createModel(config),
      systemPrompt: `You are a Goal Extractor. Your job is to take a candidate goal and a set of sub-ideas and produce a final, measurable goal plus 1-5 explicit acceptance criteria.

Rules:
- The final "goal" must be testable: a system can mark it achieved or not achieved without ambiguity
- Each acceptance criterion must be observable: you can write a test that returns pass/fail
- Avoid vague words like "good", "nice", "well", "correctly" — be specific
- 1-5 criteria only. If you have more, combine the weakest ones

Output a JSON object matching the requested schema.`,
      structuredOutputSchema: GoalSpecSchema,
    });
  }

  async extractGoal(candidateGoal: string, subIdeaNames: string[]): Promise<GoalSpec> {
    const prompt = `# Candidate goal

${candidateGoal}

# Sub-ideas involved

${subIdeaNames.map((n) => `- ${n}`).join('\n')}

# Task

Produce the final measurable goal + 1-5 acceptance criteria. Output JSON matching the schema.`;
    const result = await this.agent.invoke(prompt);
    return GoalSpecSchema.parse(JSON.parse(extractText(result)));
  }
}