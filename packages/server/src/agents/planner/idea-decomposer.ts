import { Agent } from '@strands-agents/sdk';
import { z } from 'zod';
import type { AppConfig } from '@promptsheon/shared';
import { createModel } from '../model.js';
import { extractText } from '../utils.js';
import { IdeaDecompositionSchema, type IdeaDecomposition } from './types.js';

/**
 * IdeaDecomposer agent: takes a free-form user idea and produces 2-7
 * sub-ideas + a candidate goal. Uses Strands structured output (Zod schema)
 * to guarantee the shape of the response.
 */
export class IdeaDecomposerAgent {
  private agent: Agent;

  constructor(config: AppConfig) {
    this.agent = new Agent({
      id: 'ideaDecomposer',
      model: createModel(config),
      systemPrompt: `You are an Idea Decomposer. Your job is to take a user's free-form idea and identify the 2-7 distinct sub-ideas that, when combined, will achieve it.

Rules:
- Each sub-idea must be a single, specialised concern (not overlapping with others)
- Sub-ideas should be ordered: think about which must run before which
- Give each sub-idea a stable, lowercase id (e.g. "research", "draft", "review", "publish")
- The "suggestedPrompt" should be a starting system prompt draft for the agent that will handle this sub-idea
- The goalCandidate is a single-sentence first draft — the GoalExtractor will refine it

Output a JSON object matching the requested schema.`,
      structuredOutputSchema: IdeaDecompositionSchema,
    });
  }

  async decompose(idea: string, constraints?: string[]): Promise<IdeaDecomposition> {
    const prompt = `# User idea

${idea}

${constraints && constraints.length > 0 ? `# Constraints\n${constraints.map((c) => `- ${c}`).join('\n')}\n` : ''}
# Task

Decompose this idea into 2-7 sub-ideas. Output JSON matching the schema.`;
    const result = await this.agent.invoke(prompt);
    return IdeaDecompositionSchema.parse(JSON.parse(extractText(result)));
  }
}