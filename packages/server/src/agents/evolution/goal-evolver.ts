import { z } from 'zod';
import type { AppConfig, CasStore, Manifest } from '@promptsheon/shared';
import { Agent, DefaultModelRetryStrategy, ExponentialBackoff } from '@strands-agents/sdk';
import { createModel } from '../model.js';
import { extractText } from '../utils.js';
import { ManifestGraphExecutor, validateDag } from '../executor/index.js';
import { SseHub } from '../../sse/hub.js';
import { StrandsEvaluatorAdapter } from '../evaluation/evaluator-adapter.js';
import { EvalSuiteRunner } from '../evaluation/suite-runner.js';
import { EvaluatorRegistry, EVALUATOR_NAMES } from '../evaluation/registry.js';
import type { EvaluatorName } from '../evaluation/registry.js';

export interface EvolutionSnapshot {
  iteration: number;
  manifestHash: string;
  manifest: Manifest;
  score: number;
  timestamp: string;
}

export interface EvolutionOptions {
  maxIterations: number;
  cooldownMs: number;
  costBudget: number;
}

export interface IterationRecord {
  iteration: number;
  score: number;
  cost: number;
  nodeId?: string;
  snapshotId?: string;
  revised: boolean;
  timestamp: string;
}

export interface EvolutionSnapshot {
  iteration: number;
  manifestHash: string;
  manifest: Manifest;
  score: number;
  timestamp: string;
}

export interface EvolutionResult {
  passed: boolean;
  manifestHash: string;
  bestScore: number;
  bestManifestHash: string;
  iterations: number;
  totalCost: number;
  history: IterationRecord[];
  error?: string;
}

const RevisionSchema = z.object({
  revisedSubManifest: z.object({
    systemPrompt: z.string().min(1).describe('Revised system prompt for the weakest sub-capability'),
  }).passthrough(),
  changes: z.array(z.string()).describe('Human-readable list of changes made'),
  reasoning: z.string().describe('Why these changes should help achieve the goal'),
});

type Revision = z.infer<typeof RevisionSchema>;

/**
 * GoalBasedEvolutionAgent — iterative loop that runs the Manifest DAG,
 * evaluates it against the eval suite, and revises the weakest node until
 * passThreshold is met.
 *
 * Iteration loop:
 * 1. Execute manifest via ManifestGraphExecutor
 * 2. Run eval suite via injected evaluator (LLM-judge)
 * 3. If score >= passThreshold, return success
 * 4. Else: identify weakest node, generate revised sub-manifest via
 *    Strands structured output (RevisionSchema)
 * 5. Save snapshot for rollback
 * 6. Apply revision, re-execute, repeat
 *
 * Hard caps (per options): maxIterations, cooldownMs, costBudget
 */
export class GoalBasedEvolutionAgent {
  private revisionAgent: Agent;
  private state = new Map<string, GoalEvolutionState>();
  private evaluatorRegistry: EvaluatorRegistry;

  constructor(
    private deps: { config: AppConfig; hub: SseHub; executor: ManifestGraphExecutor; cas: CasStore },
  ) {
    this.revisionAgent = new Agent({
      id: 'goalRevisioner',
      model: createModel(deps.config),
      systemPrompt: `You are a Manifest Revision agent. Your job is to improve a single sub-capability's system prompt based on evaluation results.

Given:
- The current sub-manifest (system prompt, model, tools, guardrails)
- The parent goal + acceptance criteria
- Recent eval results (node-level scores)
- Failure cases (inputs + expected outputs + actual outputs)

You produce a JSON object matching the schema:
- revisedSubManifest.systemPrompt: the new prompt
- changes: list of changes you made
- reasoning: why this should achieve the goal

Be conservative: small targeted edits, preserve what works.`,
      structuredOutputSchema: RevisionSchema,
      retryStrategy: new DefaultModelRetryStrategy({
        maxAttempts: 3,
        backoff: new ExponentialBackoff({ baseMs: 1000, maxMs: 10000 }),
      }),
    });
    this.evaluatorRegistry = new EvaluatorRegistry(deps.config);
  }

  async evolve(manifestHash: string, manifest: Manifest, options: EvolutionOptions): Promise<EvolutionResult> {
    const validation = validateDag(manifest);
    if (!validation.valid) {
      return {
        passed: false,
        manifestHash,
        bestScore: 0,
        bestManifestHash: manifestHash,
        iterations: 0,
        totalCost: 0,
        history: [],
        error: `invalid DAG: ${validation.errors.join('; ')}`,
      };
    }

    let currentManifest: Manifest = manifest;
    let currentHash = manifestHash;
    let bestManifest: Manifest = currentManifest;
    let bestManifestHash = currentHash;
    let bestScore = 0;
    let totalCost = 0;
    let lastError: string | undefined;
    const history: IterationRecord[] = [];
    const snapshots = new Map<number, EvolutionSnapshot>();
    snapshots.set(0, {
      iteration: 0,
      manifestHash: currentHash,
      manifest: currentManifest,
      score: 0,
      timestamp: new Date().toISOString(),
    });

    const executionIdBase = `evol-${Date.now()}`;

    for (let i = 0; i < options.maxIterations; i++) {
      if (totalCost >= options.costBudget) {
        lastError = `cost budget ${options.costBudget} exceeded`;
        break;
      }

      this.deps.hub.broadcast({
        type: 'status',
        data: { kind: 'evolution_iteration', iteration: i + 1, manifestHash: currentHash },
        timestamp: new Date().toISOString(),
      });

      const trace = await this.deps.executor.execute(currentHash, currentManifest, {
        executionId: `${executionIdBase}-${i + 1}`,
        inputs: {},
        environment: 'dev',
      });

      const iterCost = trace.totalCost;
      totalCost += iterCost;

      const score = await this.scoreAgainstGoal(trace, currentManifest);
      const passed = score >= currentManifest.evaluation.passThreshold;

      history.push({
        iteration: i + 1,
        score,
        cost: iterCost,
        timestamp: new Date().toISOString(),
        revised: false,
      });
      snapshots.set(i + 1, {
        iteration: i + 1,
        manifestHash: currentHash,
        manifest: currentManifest,
        score,
        timestamp: new Date().toISOString(),
      });

      if (passed) {
        this.deps.hub.broadcast({
          type: 'status',
          data: { kind: 'evolution_passed', iteration: i + 1, score },
          timestamp: new Date().toISOString(),
        });
        this.state.set(manifestHash, {
          currentHash,
          bestHash: currentHash,
          bestScore: score,
          iteration: i + 1,
        });
        return {
          passed: true,
          manifestHash: currentHash,
          bestScore: score,
          bestManifestHash: currentHash,
          iterations: i + 1,
          totalCost,
          history,
        };
      }

      if (score > bestScore) {
        bestScore = score;
        bestManifest = currentManifest;
        bestManifestHash = currentHash;
      }

      const weakest = this.findWeakestNode(trace);
      if (!weakest) {
        lastError = 'no weakest node identified';
        break;
      }

      history[history.length - 1].nodeId = weakest;

      try {
        const revised = await this.reviseNode(currentManifest, weakest, score);
        const nextManifest: Manifest = this.applyRevision(currentManifest, weakest, revised);
        currentManifest = nextManifest;
        currentHash = await this.persistManifest(nextManifest, currentHash, manifestHash);
        history[history.length - 1].revised = true;
        history[history.length - 1].snapshotId = `${currentHash}`;
      } catch (e) {
        lastError = `revision failed: ${(e as Error).message}`;
        const prev = snapshots.get(i);
        if (prev) {
          currentManifest = prev.manifest;
          currentHash = prev.manifestHash;
        }
        break;
      }

      if (options.cooldownMs > 0) {
        await new Promise((resolve) => setTimeout(resolve, options.cooldownMs));
      }
    }

    this.state.set(manifestHash, {
      currentHash,
      bestHash: bestManifestHash,
      bestScore,
      iteration: history.length,
    });
    return {
      passed: false,
      manifestHash: currentHash,
      bestScore,
      bestManifestHash,
      iterations: history.length,
      totalCost,
      history,
      error: lastError,
    };
  }

  private async scoreAgainstGoal(
    trace: { nodeResults: Record<string, { output: string; status: string }> },
    manifest: Manifest,
  ): Promise<number> {
    const nodeOutputs = Object.values(trace.nodeResults)
      .map((n) => n.output)
      .filter((o) => o && o.length > 0);
    if (nodeOutputs.length === 0) return 0;
    const actual = nodeOutputs.join('\n\n');
    const goal = this.resolveGoal(manifest);

    const declaredScorers = manifest.evaluation.scorers.filter((s): s is EvaluatorName =>
      EVALUATOR_NAMES.includes(s as EvaluatorName),
    );

    try {
      // Multi-scorer path: run every declared scorer and aggregate.
      if (declaredScorers.length > 1) {
        const runner = new EvalSuiteRunner(this.deps.config);
        const suiteResult = await runner.run(manifest, {
          actual,
          expected: goal,
          inputs: { goal },
        });
        return suiteResult.aggregateScore;
      }

      // Single-scorer path: use the resolved primary scorer.
      const scorerName = this.resolveScorerName(manifest);
      const adapter = new StrandsEvaluatorAdapter(this.deps.config, scorerName);
      const result = await adapter.score({
        actual,
        expected: goal,
        inputs: { goal },
        manifestHash: manifest.id,
      });
      return result.score;
    } catch {
      const nodeCount = Object.keys(trace.nodeResults).length;
      const completed = Object.values(trace.nodeResults).filter((n) => n.status === 'completed').length;
      return nodeCount === 0 ? 0 : completed / nodeCount;
    }
  }

  private resolveScorerName(manifest: Manifest): EvaluatorName {
    const meta = manifest.metadata['primaryScorer'];
    if (typeof meta === 'string' && EVALUATOR_NAMES.includes(meta as EvaluatorName)) {
      return meta as EvaluatorName;
    }
    const declared = manifest.evaluation.scorers[0];
    if (typeof declared === 'string' && EVALUATOR_NAMES.includes(declared as EvaluatorName)) {
      return declared as EvaluatorName;
    }
    return 'goal-success-rate';
  }

  private resolveGoal(manifest: Manifest): string {
    const meta = manifest.metadata['goal'];
    if (typeof meta === 'string') return meta;
    return `passThreshold ${manifest.evaluation.passThreshold}`;
  }

  private findWeakestNode(trace: { nodeResults: Record<string, { latencyMs: number; status: string; error: string }> }): string | null {
    const candidates = Object.entries(trace.nodeResults)
      .filter(([, n]) => n.status === 'failed' || n.error)
      .sort((a, b) => b[1].latencyMs - a[1].latencyMs);
    if (candidates.length === 0) {
      const all = Object.keys(trace.nodeResults);
      return all[0] ?? null;
    }
    return candidates[0]?.[0] ?? null;
  }

  private async reviseNode(manifest: Manifest, nodeId: string, currentScore: number): Promise<Revision> {
    const node = manifest.nodes.find((n) => n.id === nodeId);
    if (!node) throw new Error(`node ${nodeId} not found`);

    if (this.reviseOverride) {
      return this.reviseOverride(manifest, nodeId, currentScore);
    }

    const prompt = `# Parent goal

${manifest.evaluation.passThreshold ? 'Goal score >= ' + manifest.evaluation.passThreshold.toString() : 'Goal: ' + (manifest.metadata['goal'] as string ?? 'achieve the user intent')}

# Sub-capability to revise

${node.id} (${node.name}): ${node.goal}

Current system prompt:
${node.manifest.prompt.systemPrompt}

# Current score: ${currentScore.toFixed(2)}

# Task

Produce a revised sub-manifest with an improved system prompt. Output JSON matching the schema.`;

    const result = await this.revisionAgent.invoke(prompt);
    return RevisionSchema.parse(JSON.parse(extractText(result)));
  }

  setReviseOverride(fn: (manifest: Manifest, nodeId: string, score: number) => Promise<Revision>): void {
    this.reviseOverride = fn;
  }

  private reviseOverride: ((manifest: Manifest, nodeId: string, score: number) => Promise<Revision>) | null = null;

  private applyRevision(manifest: Manifest, nodeId: string, revision: Revision): Manifest {
    return {
      ...manifest,
      nodes: manifest.nodes.map((n) =>
        n.id === nodeId
          ? {
              ...n,
              manifest: {
                ...n.manifest,
                prompt: {
                  ...n.manifest.prompt,
                  systemPrompt: revision.revisedSubManifest.systemPrompt,
                },
              },
            }
          : n,
      ),
      metadata: {
        ...manifest.metadata,
        lastRevision: {
          nodeId,
          changes: revision.changes,
          reasoning: revision.reasoning,
          at: new Date().toISOString(),
        },
      },
    };
  }

  private async persistManifest(manifest: Manifest, hash: string, _parentHash: string): Promise<string> {
    void hash;
    void _parentHash;
    return this.deps.cas.writeObject({
      type: 'blob',
      data: Buffer.from(JSON.stringify(manifest)),
    });
  }

  getState(key: string): GoalEvolutionState | undefined {
    return this.state.get(key);
  }

  /** Returns the manifest at the start of a given iteration (0-indexed). */
  getSnapshot(iteration: number, currentManifest: Manifest, currentHash: string, score: number): EvolutionSnapshot {
    return { iteration, manifestHash: currentHash, manifest: currentManifest, score, timestamp: new Date().toISOString() };
  }

  /** Returns the current state for a given manifest hash, if any. */
  getCurrentState(hash: string): { currentHash: string; bestHash: string; bestScore: number; iteration: number } | undefined {
    const s = this.state.get(hash);
    if (!s) return undefined;
    return { currentHash: s.currentHash, bestHash: s.bestHash, bestScore: s.bestScore, iteration: s.iteration };
  }
}

interface GoalEvolutionState {
  currentHash: string;
  bestHash: string;
  bestScore: number;
  iteration: number;
}