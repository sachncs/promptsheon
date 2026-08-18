import type { AppConfig, Capability, EvalRun } from '@promptsheon/shared';
import type { CasStore } from '@promptsheon/shared';
import { Agent } from '@strands-agents/sdk';
import { createModel } from '../model.js';
import { extractText } from '../utils.js';

export interface SelfEvolveState {
  status: 'idle' | 'detected' | 'revising' | 'validating' | 'promoted' | 'rejected';
  lastRevisionHash: string;
  lastEvalScore: number;
  cycleCount: number;
}

export class PerformanceDetector {
  detect(
    capability: Capability,
    recentEvals: EvalRun[],
  ): { detected: boolean; score: number; threshold: number } {
    if (recentEvals.length === 0) {
      return { detected: false, score: 1, threshold: capability.selfEvolveMinScore };
    }

    const latest = recentEvals[0];
    const score = latest.score;
    return {
      detected: score < capability.selfEvolveMinScore,
      score,
      threshold: capability.selfEvolveMinScore,
    };
  }
}

export interface ReviseRequest {
  currentManifest: { systemPrompt: string; tools: unknown[]; parameters: Record<string, unknown> };
  failingCases: Array<{ inputs: unknown; expected: unknown; actual: unknown }>;
  evaluationSummary: string;
}

export interface ReviseResponse {
  revisedManifest: { systemPrompt: string; tools: unknown[]; parameters: Record<string, unknown> };
  changes: string[];
  reasoning: string;
}

export class LLMRevisionStrategy {
  private agent: Agent;

  constructor(config: AppConfig) {
    this.agent = new Agent({
      model: createModel(config),
      systemPrompt: `You are a prompt revision agent. Your job is to improve a prompt based on evaluation failures.

Given:
- The current manifest (system prompt, tools, parameters)
- Failing test cases with inputs, expected outputs, and actual outputs
- An evaluation summary

Revise the manifest to fix the failures while preserving correct behavior.

Output a JSON object with:
- revisedManifest: the improved manifest
- changes: list of changes made
- reasoning: explanation of why changes were made`,
    });
  }

  async revise(request: ReviseRequest): Promise<ReviseResponse> {
    const result = await this.agent.invoke(JSON.stringify(request));
    return JSON.parse(extractText(result));
  }
}

export class CasPromptLoader {
  constructor(private cas: CasStore) {}

  async load(hash: string): Promise<{ systemPrompt: string; tools: unknown[]; parameters: Record<string, unknown> }> {
    const obj = await this.cas.readObject(hash);
    if (obj.type !== 'blob') throw new Error('expected blob');
    return JSON.parse(obj.data.toString());
  }

  async save(manifest: { systemPrompt: string; tools: unknown[]; parameters: Record<string, unknown> }): Promise<string> {
    const data = Buffer.from(JSON.stringify(manifest));
    return this.cas.writeObject({ type: 'blob', data });
  }
}

export class EvolutionAgent {
  private detector: PerformanceDetector;
  private reviser: LLMRevisionStrategy;
  private loader: CasPromptLoader;
  private state = new Map<string, SelfEvolveState>();

  constructor(
    config: AppConfig,
    deps: { cas: CasStore },
  ) {
    this.detector = new PerformanceDetector();
    this.reviser = new LLMRevisionStrategy(config);
    this.loader = new CasPromptLoader(deps.cas);
  }

  async runCycle(
    capabilityId: string,
    manifestHash: string,
    recentEvals: EvalRun[],
    capability: Capability,
  ): Promise<{ action: 'revised' | 'no_change'; state: SelfEvolveState }> {
    const current = await this.loader.load(manifestHash);

    const detection = this.detector.detect(capability, recentEvals);
    if (!detection.detected) {
      const state: SelfEvolveState = { status: 'idle', lastRevisionHash: manifestHash, lastEvalScore: detection.score, cycleCount: 0 };
      this.state.set(capabilityId, state);
      return { action: 'no_change', state };
    }

    const revised = await this.reviser.revise({
      currentManifest: current,
      failingCases: [],
      evaluationSummary: `Score: ${detection.score}, threshold: ${detection.threshold}`,
    });

    const newHash = await this.loader.save(revised.revisedManifest);
    const existing = this.state.get(capabilityId);
    const state: SelfEvolveState = {
      status: 'promoted',
      lastRevisionHash: newHash,
      lastEvalScore: detection.score,
      cycleCount: (existing?.cycleCount ?? 0) + 1,
    };
    this.state.set(capabilityId, state);
    return { action: 'revised', state };
  }

  getState(capabilityId: string): SelfEvolveState | undefined {
    return this.state.get(capabilityId);
  }
}
