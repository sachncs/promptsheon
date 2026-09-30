import type { AppConfig, Capability, EvalRun } from '@promptsheon/shared';
import type { CasStore } from '@promptsheon/shared';
import { Agent } from '@strands-agents/sdk';
import { z } from 'zod';
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

const ManifestSchema = z.object({
  systemPrompt: z.string(),
  tools: z.array(z.unknown()),
  parameters: z.record(z.string(), z.unknown()),
}).passthrough();
const ManifestRevisionSchema = z.object({ revisedManifest: ManifestSchema });

export class EvolutionAgent {
  private revisionAgent: Agent;
  private state = new Map<string, SelfEvolveState>();

  constructor(private config: AppConfig, private deps: { cas: CasStore }) {
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

    const revised = this.config.llm.defaultProvider === 'simulated'
      ? {
          revisedManifest: {
            ...current,
            systemPrompt: `${current.systemPrompt}\n\nSimulation refinement: preserve successful behaviour and clarify the observed failing case.`,
          },
        }
      : await this.reviseManifest(current, score, threshold);

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

  private async reviseManifest(current: Manifest, score: number, threshold: number): Promise<{ revisedManifest: Manifest }> {
    const result = await this.revisionAgent.invoke(JSON.stringify({
      currentManifest: current,
      failingCases: [],
      evaluationSummary: `Score: ${score}, threshold: ${threshold}`,
    }));
    let parsed: unknown;
    try {
      parsed = JSON.parse(extractText(result));
    } catch (error) {
      throw new Error('revision agent returned invalid JSON', { cause: error });
    }
    const revision = ManifestRevisionSchema.safeParse(parsed);
    if (!revision.success) throw new Error('revision agent returned an invalid manifest');
    return revision.data;
  }

  private async loadManifest(hash: string): Promise<Manifest> {
    const obj = await this.deps.cas.readObject(hash);
    if (obj.type !== 'blob') throw new Error('expected blob');
    let parsed: unknown;
    try {
      parsed = JSON.parse(obj.data.toString());
    } catch (error) {
      throw new Error('stored evolution manifest is invalid JSON', { cause: error });
    }
    const manifest = ManifestSchema.safeParse(parsed);
    if (!manifest.success) throw new Error('stored evolution manifest has an invalid shape');
    return manifest.data;
  }

  private async saveManifest(manifest: Manifest): Promise<string> {
    return this.deps.cas.writeObject({ type: 'blob', data: Buffer.from(JSON.stringify(manifest)) });
  }
}
