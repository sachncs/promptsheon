import { Agent, Swarm } from '@strands-agents/sdk';
import { z } from 'zod';
import type { AppConfig } from '@promptsheon/shared';
import { createModel } from '../model.js';
import { extractText } from '../utils.js';
import {
  IdeaDecompositionSchema,
  GoalSpecSchema,
  type IdeaInput,
  type PlannedDAG,
} from './types.js';
import { IdeaDecomposerAgent } from './idea-decomposer.js';
import { GoalExtractorAgent } from './goal-extractor.js';

/**
 * Internal structured output schema for the DAG builder agent.
 */
const DagStructureSchema = z.object({
  nodes: z.array(z.object({
    id: z.string().min(1),
    name: z.string().min(1),
    description: z.string(),
    goal: z.string().min(1),
    suggestedPrompt: z.string(),
  })),
  edges: z.array(z.object({
    from: z.string().min(1),
    to: z.string().min(1),
    mapping: z.record(z.string(), z.string()).default({}),
  })),
});

type DagStructure = z.infer<typeof DagStructureSchema>;

/**
 * Internal structured output schema for the eval synthesizer agent.
 */
const EvalSuiteSchema = z.object({
  syntheticCases: z.array(z.object({
    input: z.unknown(),
    expected: z.unknown(),
  })).min(3).max(10),
  passThreshold: z.number().min(0).max(1).default(0.7),
});

type EvalSuite = z.infer<typeof EvalSuiteSchema>;

/**
 * IdeaPlannerAgent — Strands Swarm that orchestrates 4 specialised agents
 * to decompose a free-form user idea into a complete Manifest DAG:
 *
 *   1. IdeaDecomposer  → sub-ideas + candidate goal
 *   2. GoalExtractor   → measurable goal + acceptance criteria
 *   3. DagBuilder      → nodes + edges (data flow)
 *   4. EvalSynthesizer → synthetic eval cases + pass threshold
 *
 * Returns a PlannedDAG. If any step produces invalid output, the planner
 * falls back to a single-node Manifest with the original idea as the goal.
 */
export class IdeaPlannerAgent {
  private decomposer: IdeaDecomposerAgent;
  private goalExtractor: GoalExtractorAgent;
  private dagBuilder: Agent;
  private evalSynthesizer: Agent;
  private swarm: Swarm;
  private fallbackDag: PlannedDAG;

  constructor(config: AppConfig) {
    this.decomposer = new IdeaDecomposerAgent(config);
    this.goalExtractor = new GoalExtractorAgent(config);

    this.dagBuilder = new Agent({
      id: 'dagBuilder',
      model: createModel(config),
      systemPrompt: `You are a DAG Builder. Given a goal + sub-ideas, produce a directed acyclic graph of sub-capabilities.

Rules:
- One node per sub-idea
- Edges represent data flow: an edge from A to B means A's output feeds into B's input
- Use descriptive mapping: e.g. {"research_output": "draft_input"} — field names are illustrative
- If sub-ideas can run in parallel, do NOT add edges between them (DAG is sparse)
- Output JSON matching the schema.`,
      structuredOutputSchema: DagStructureSchema,
    });

    this.evalSynthesizer = new Agent({
      id: 'evalSynthesizer',
      model: createModel(config),
      systemPrompt: `You are an Eval Synthesizer. Given a goal + sub-ideas, produce 3-10 synthetic test cases that together prove the goal is achievable.

Each test case has:
- input: a realistic input the user might provide
- expected: what the correct output would look like (in natural language or JSON shape)

Pick passThreshold between 0.5 and 0.9 based on goal complexity:
- 0.5-0.6 for creative / open-ended goals
- 0.7 for typical analytical tasks
- 0.8-0.9 for tightly specified factual tasks

Output JSON matching the schema.`,
      structuredOutputSchema: EvalSuiteSchema,
    });

    this.swarm = new Swarm({
      nodes: [this.decomposer as unknown as Agent, this.goalExtractor as unknown as Agent, this.dagBuilder, this.evalSynthesizer],
      maxSteps: 10,
    });

    this.fallbackDag = this.buildFallbackDag('');
  }

  /**
   * Plan a Manifest DAG from a free-form idea. Returns a PlannedDAG
   * describing the goal, nodes, edges, and synthetic eval suite.
   *
   * On any failure (LLM error, invalid output), returns a single-node
   * fallback DAG with the idea as the goal.
   */
  async plan(input: IdeaInput): Promise<PlannedDAG> {
    try {
      const decomposition = await this.decomposer.decompose(input.idea, input.constraints);

      const goalSpec = await this.goalExtractor.extractGoal(
        decomposition.goalCandidate,
        decomposition.subIdeas.map((s) => s.name),
      );

      const dagPrompt = `# Goal\n${goalSpec.goal}\n\n# Sub-ideas\n${decomposition.subIdeas.map((s) => `- ${s.id} (${s.name}): ${s.description}\n  suggestedPrompt: ${s.suggestedPrompt}`).join('\n')}\n\n# Acceptance criteria\n${goalSpec.acceptanceCriteria.map((c, i) => `${i + 1}. ${c}`).join('\n')}\n\n# Task\nBuild the DAG (nodes + edges) connecting these sub-ideas. Output JSON matching the schema.`;
      const dagResult = await this.dagBuilder.invoke(dagPrompt);
      const dagStructure: DagStructure = DagStructureSchema.parse(JSON.parse(extractText(dagResult)));

      const evalPrompt = `# Goal\n${goalSpec.goal}\n\n# Sub-ideas\n${dagStructure.nodes.map((n) => `- ${n.id} (${n.name}): ${n.goal}`).join('\n')}\n\n# Acceptance criteria\n${goalSpec.acceptanceCriteria.map((c, i) => `${i + 1}. ${c}`).join('\n')}\n\n# Task\nProduce 3-10 synthetic test cases + passThreshold. Output JSON matching the schema.`;
      const evalResult = await this.evalSynthesizer.invoke(evalPrompt);
      const evalSuite: EvalSuite = EvalSuiteSchema.parse(JSON.parse(extractText(evalResult)));

      return {
        goal: goalSpec.goal,
        acceptanceCriteria: goalSpec.acceptanceCriteria,
        nodes: dagStructure.nodes,
        edges: dagStructure.edges,
        syntheticCases: evalSuite.syntheticCases,
        passThreshold: evalSuite.passThreshold,
      };
    } catch (e) {
      console.error('IdeaPlannerAgent.plan failed, returning fallback:', e);
      return { ...this.fallbackDag, nodes: [{ id: 'root', name: 'Root', description: input.idea, goal: input.idea, suggestedPrompt: `You handle: ${input.idea}` }] };
    }
  }

  private buildFallbackDag(idea: string): PlannedDAG {
    return {
      goal: idea || 'Process input',
      acceptanceCriteria: ['Produces a non-empty response'],
      nodes: [],
      edges: [],
      syntheticCases: [],
      passThreshold: 0.5,
    };
  }
}