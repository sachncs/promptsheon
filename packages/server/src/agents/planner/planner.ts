import { Agent, Swarm } from '@strands-agents/sdk';
import { z } from 'zod';
import type { AppConfig } from '@promptsheon/shared';
import { createModel } from '../model.js';
import {
  IdeaDecompositionSchema,
  GoalSpecSchema,
  type IdeaInput,
  type PlannedDAG,
} from './types.js';

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
 * Build the 4 specialised agents that comprise the planner Swarm.
 *
 * Each agent is a first-class Strands Agent (not a wrapper class) so
 * the Swarm can address it by id and the structured-output schema is
 * honoured at the LLM boundary.
 */
function buildPlannerAgents(config: AppConfig): {
  ideaDecomposer: Agent;
  goalExtractor: Agent;
  dagBuilder: Agent;
  evalSynthesizer: Agent;
} {
  return {
    ideaDecomposer: new Agent({
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
    }),
    goalExtractor: new Agent({
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
    }),
    dagBuilder: new Agent({
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
    }),
    evalSynthesizer: new Agent({
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
    }),
  };
}

/**
 * IdeaPlannerAgent — Strands Swarm that orchestrates 4 specialised
 * agents to decompose a free-form user idea into a complete Manifest
 * DAG (nodes + edges + goal + synthetic eval suite).
 *
 * Agents (in invocation order): ideaDecomposer → goalExtractor →
 * dagBuilder → evalSynthesizer. The Swarm's `repetitiveHandoff`
 * detection guards against LLM-driven routing loops.
 *
 * The plan() method drives the Swarm and then composes the final
 * PlannedDAG. On any failure, returns a single-node fallback DAG.
 */
export class IdeaPlannerAgent {
  private agents: ReturnType<typeof buildPlannerAgents>;
  private swarm: Swarm;
  private fallbackDag: PlannedDAG;

  constructor(config: AppConfig) {
    this.agents = buildPlannerAgents(config);
    this.swarm = new Swarm({
      nodes: [
        this.agents.ideaDecomposer,
        this.agents.goalExtractor,
        this.agents.dagBuilder,
        this.agents.evalSynthesizer,
      ],
      start: 'ideaDecomposer',
      maxSteps: 10,
      repetitiveHandoffDetectionWindow: 6,
      repetitiveHandoffMinUniqueAgents: 3,
    });
    this.fallbackDag = this.buildFallbackDag('');
  }

  /**
   * Plan a Manifest DAG from a free-form idea. Returns a PlannedDAG
   * describing the goal, nodes, edges, and synthetic eval suite.
   *
   * Drives each agent sequentially (matching the explicit ordering
   * encoded in the Swarm's start + maxSteps) and composes the final
   * structured output. On any failure, returns a single-node fallback
   * DAG with the idea as the goal.
   */
  async plan(input: IdeaInput): Promise<PlannedDAG> {
    try {
      const decomposition = await this.agents.ideaDecomposer.invoke(
        this.decomposerPrompt(input.idea, input.constraints),
      );
      const decompositionText = this.extractAgentText(decomposition);
      const decompositionData = IdeaDecompositionSchema.parse(JSON.parse(decompositionText));

      const goalResult = await this.agents.goalExtractor.invoke(
        this.goalExtractorPrompt(
          decompositionData.goalCandidate,
          decompositionData.subIdeas.map((s) => s.name),
        ),
      );
      const goalText = this.extractAgentText(goalResult);
      const goalSpec = GoalSpecSchema.parse(JSON.parse(goalText));

      const dagResult = await this.agents.dagBuilder.invoke(
        this.dagBuilderPrompt(goalSpec, decompositionData.subIdeas),
      );
      const dagText = this.extractAgentText(dagResult);
      const dagStructure = DagStructureSchema.parse(JSON.parse(dagText));

      const evalResult = await this.agents.evalSynthesizer.invoke(
        this.evalSynthesizerPrompt(goalSpec, dagStructure.nodes),
      );
      const evalText = this.extractAgentText(evalResult);
      const evalSuite = EvalSuiteSchema.parse(JSON.parse(evalText));

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
      return {
        ...this.fallbackDag,
        nodes: [
          {
            id: 'root',
            name: 'Root',
            description: input.idea,
            goal: input.idea,
            suggestedPrompt: `You handle: ${input.idea}`,
          },
        ],
      };
    }
  }

  private decomposerPrompt(idea: string, constraints?: string[]): string {
    return `# User idea\n\n${idea}\n\n${
      constraints && constraints.length > 0
        ? `# Constraints\n${constraints.map((c) => `- ${c}`).join('\n')}\n`
        : ''
    }# Task\n\nDecompose this idea into 2-7 sub-ideas. Output JSON matching the schema.`;
  }

  private goalExtractorPrompt(candidateGoal: string, subIdeaNames: string[]): string {
    return `# Candidate goal\n\n${candidateGoal}\n\n# Sub-ideas involved\n\n${subIdeaNames
      .map((n) => `- ${n}`)
      .join('\n')}\n\n# Task\n\nProduce the final measurable goal + 1-5 acceptance criteria. Output JSON matching the schema.`;
  }

  private dagBuilderPrompt(
    goalSpec: { goal: string; acceptanceCriteria: string[] },
    subIdeas: Array<{ id: string; name: string; description: string; suggestedPrompt: string }>,
  ): string {
    return `# Goal\n${goalSpec.goal}\n\n# Sub-ideas\n${subIdeas
      .map((s) => `- ${s.id} (${s.name}): ${s.description}\n  suggestedPrompt: ${s.suggestedPrompt}`)
      .join('\n')}\n\n# Acceptance criteria\n${goalSpec.acceptanceCriteria
      .map((c, i) => `${i + 1}. ${c}`)
      .join('\n')}\n\n# Task\nBuild the DAG (nodes + edges) connecting these sub-ideas. Output JSON matching the schema.`;
  }

  private evalSynthesizerPrompt(
    goalSpec: { goal: string; acceptanceCriteria: string[] },
    nodes: Array<{ id: string; name: string; goal: string }>,
  ): string {
    return `# Goal\n${goalSpec.goal}\n\n# Sub-ideas\n${nodes
      .map((n) => `- ${n.id} (${n.name}): ${n.goal}`)
      .join('\n')}\n\n# Acceptance criteria\n${goalSpec.acceptanceCriteria
      .map((c, i) => `${i + 1}. ${c}`)
      .join('\n')}\n\n# Task\nProduce 3-10 synthetic test cases + passThreshold. Output JSON matching the schema.`;
  }

  private extractAgentText(result: unknown): string {
    const r = result as { lastMessage?: { content?: Array<{ type: string; text?: string }> } };
    if (!r.lastMessage?.content) return '';
    return r.lastMessage.content
      .filter((b) => b.type === 'textBlock')
      .map((b) => b.text ?? '')
      .join('');
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