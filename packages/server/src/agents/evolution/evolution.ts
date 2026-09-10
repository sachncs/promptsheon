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

interface Manifest {
  systemPrompt: string;
  tools: unknown[];
  parameters: Record<string, unknown>;
}

export class EvolutionAgent {
  private revisionAgent: Agent;
  private state = new Map<string, SelfEvolveState>();

  constructor(config: AppConfig, private deps: { cas: CasStore }) {
    this.revisionAgent = new Agent({
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

  async runCycle(
    capabilityId: string,
    manifestHash: string,
    recentEvals: EvalRun[],
    capability: Capability,
  ): Promise<{ action: 'revised' | 'no_change'; state: SelfEvolveState }> {
    const current = await this.loadManifest(manifestHash);

    const score = recentEvals[0]?.score ?? 1;
    const threshold = capability.selfEvolveMinScore;
    if (recentEvals.length === 0 || score >= threshold) {
      const state: SelfEvolveState = { status: 'idle', lastRevisionHash: manifestHash, lastEvalScore: score, cycleCount: 0 };
      this.state.set(capabilityId, state);
      return { action: 'no_change', state };
    }

    const result = await this.revisionAgent.invoke(JSON.stringify({
      currentManifest: current,
      failingCases: [],
      evaluationSummary: `Score: ${score}, threshold: ${threshold}`,
    }));
    const revised = JSON.parse(extractText(result)) as { revisedManifest: Manifest };

    const newHash = await this.saveManifest(revised.revisedManifest);
    const existing = this.state.get(capabilityId);
    const state: SelfEvolveState = {
      status: 'promoted',
      lastRevisionHash: newHash,
      lastEvalScore: score,
      cycleCount: (existing?.cycleCount ?? 0) + 1,
    };
    this.state.set(capabilityId, state);
    return { action: 'revised', state };
  }

  getState(capabilityId: string): SelfEvolveState | undefined {
    return this.state.get(capabilityId);
  }

  private async loadManifest(hash: string): Promise<Manifest> {
    const obj = await this.deps.cas.readObject(hash);
    if (obj.type !== 'blob') throw new Error('expected blob');
    return JSON.parse(obj.data.toString());
  }

  private async saveManifest(manifest: Manifest): Promise<string> {
    return this.deps.cas.writeObject({ type: 'blob', data: Buffer.from(JSON.stringify(manifest)) });
  }
}