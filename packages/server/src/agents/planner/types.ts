import { z } from 'zod';

/**
 * A single sub-idea produced by the IdeaDecomposer agent.
 */
export const SubIdeaSchema = z.object({
  id: z.string().min(1).describe('Stable identifier for the sub-idea (e.g. "research", "draft", "review")'),
  name: z.string().min(1).describe('Short human-readable name (1-5 words)'),
  description: z.string().describe('What this sub-idea is responsible for'),
  suggestedPrompt: z.string().describe('Initial system prompt draft for the agent handling this sub-idea'),
});

/**
 * Output of the IdeaDecomposer agent: 2-7 sub-ideas + a candidate goal.
 */
export const IdeaDecompositionSchema = z.object({
  subIdeas: z.array(SubIdeaSchema).min(2).max(7).describe('Sub-ideas that together achieve the user\'s idea'),
  goalCandidate: z.string().min(1).describe('Candidate top-level goal string (refined by GoalExtractor)'),
});

export type SubIdea = z.infer<typeof SubIdeaSchema>;
export type IdeaDecomposition = z.infer<typeof IdeaDecompositionSchema>;

/**
 * Output of the GoalExtractor agent: a measurable goal + acceptance criteria.
 */
export const GoalSpecSchema = z.object({
  goal: z.string().min(1).describe('Single-sentence goal, measurable via the eval suite'),
  acceptanceCriteria: z.array(z.string()).min(1).max(5).describe('Specific testable conditions that mean "goal achieved"'),
});

export type GoalSpec = z.infer<typeof GoalSpecSchema>;

/**
 * Input to the IdeaPlannerAgent.plan() method.
 */
export interface IdeaInput {
  idea: string;
  constraints?: string[];
  examples?: Array<{ input: unknown; expected: unknown }>;
}

/**
 * Final output of the IdeaPlanner: a complete Manifest DAG + goal + eval suite.
 */
export interface PlannedDAG {
  goal: string;
  acceptanceCriteria: string[];
  nodes: Array<{
    id: string;
    name: string;
    description: string;
    goal: string;
    suggestedPrompt: string;
  }>;
  edges: Array<{
    from: string;
    to: string;
    mapping: Record<string, string>;
  }>;
  syntheticCases: Array<{ input: unknown; expected: unknown }>;
  passThreshold: number;
}