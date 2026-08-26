import { randomUUID } from 'node:crypto';
import type { Execution, Manifest, ReplayDiffSummary } from '@promptsheon/shared';
import { ExecutionRepo } from '../repos/execution.js';
import { ManifestRepo } from '../repos/manifest.js';
import { TraceRepo } from '../repos/trace.js';
import type { ExecutionTrace, ManifestGraphExecutor, ExecuteOptions } from './executor/index.js';

export interface ReplayExecutor {
  execute(
    manifestHash: string,
    manifest: Manifest,
    options: ExecuteOptions,
  ): Promise<ExecutionTrace>;
}

/**
 * Replay an existing execution with its original inputs, manifest,
 * model, and environment. The new execution is linked to the
 * original via `replay_of`; the original's `replay_count` is
 * incremented; an `execution_replays` row records the lineage and
 * a per-node output diff.
 *
 * Replay is intentionally **not** deterministic: LLM outputs depend
 * on temperature, provider state, and stochastic sampling. The diff
 * is the value-add — it surfaces what changed.
 */
export class ExecutionReplayService {
  constructor(
    private executionRepo: ExecutionRepo,
    private manifestRepo: ManifestRepo,
    private traceRepo: TraceRepo,
    private executor: ReplayExecutor,
  ) {}

  async replay(
    originalId: string,
    env: { organizationId?: string } = {},
  ): Promise<ReplayResult> {
    const context = this.executionRepo.findReplayContext(originalId);
    if (!context) {
      throw new ReplayNotFoundError(originalId);
    }
    const { execution: original, manifestHash, parsedInputs } = context;
    const manifest = this.manifestRepo.findByHash(manifestHash);
    if (!manifest) {
      this.executionRepo.recordReplay({
        originalExecutionId: originalId,
        replayExecutionId: null,
        outcome: 'failed',
        inputsMatch: true,
        manifestMatch: false,
        modelMatch: true,
        environmentMatch: true,
        diffSummary: JSON.stringify({
          reason: 'manifest_missing',
          manifestHash,
        }),
      });
      throw new ReplayManifestMissingError(originalId, manifestHash);
    }

    // Pre-create the replay execution row before opening the trace so
    // that trace_runs.execution_id (a FK to executions.id) resolves.
    // The execution row carries the original's manifest link + replay
    // lineage; outputs/latency/cost are filled in after the executor
    // runs.
    const replayed = this.executionRepo.create({
      capabilityVersionId: original.capabilityVersionId,
      inputs: JSON.stringify(parsedInputs),
      inputHash: original.inputHash,
      outputs: '{}',
      model: manifest.model.modelId,
      provider: manifest.model.provider,
      latencyMs: 0,
      costUsd: 0,
      promptTokens: 0,
      completionTokens: 0,
      totalTokens: 0,
      error: '',
      traceId: randomUUID(),
      environment: original.environment,
      replayOf: original.id,
    });
    const replayExecutionId = replayed.id;

    const traceRun = this.traceRepo.startRun({
      organizationId: env.organizationId ?? 'unscoped',
      executionId: replayExecutionId,
      environment: original.environment,
      name: `replay:${manifestHash.slice(0, 12)}`,
      model: manifest.model?.modelId ?? null,
      attributes: {
        manifestHash,
        route: '/api/executions/:id/replay',
        replayOf: original.id,
      },
    });
    void traceRun;

    const trace = await this.executor.execute(manifestHash, manifest, {
      executionId: replayExecutionId,
      inputs: parsedInputs,
      environment: original.environment,
      traceId: replayExecutionId,
      traceRunId: traceRun.id,
    });
    this.traceRepo.finalize(
      traceRun.id,
      trace.status === 'completed' ? 'success' : 'error',
      { tokens: trace.totalTokens, costUsd: trace.totalCost },
    );

    const updated = this.executionRepo.updateRunResult(replayed.id, {
      outputs: JSON.stringify(trace.nodeResults),
      latencyMs: trace.totalLatencyMs,
      costUsd: trace.totalCost,
      totalTokens: trace.totalTokens,
      error: trace.error ?? '',
    });
    if (updated) {
      replayed.outputs = updated.outputs;
      replayed.latencyMs = updated.latencyMs;
      replayed.costUsd = updated.costUsd;
      replayed.totalTokens = updated.totalTokens;
      replayed.error = updated.error;
    }

    this.executionRepo.incrementReplayCount(original.id);

    const diff = this.computeDiff(original, replayed, trace);

    const outcome: 'completed' | 'diverged' =
      diff.changedNodes.length === 0 &&
      diff.addedNodes.length === 0 &&
      diff.removedNodes.length === 0
        ? 'completed'
        : 'diverged';

    this.executionRepo.recordReplay({
      originalExecutionId: original.id,
      replayExecutionId: replayed.id,
      outcome,
      inputsMatch: true,
      manifestMatch: true,
      modelMatch:
        manifest.model.modelId === original.model &&
        manifest.model.provider === original.provider,
      environmentMatch: original.environment === replayed.environment,
      diffSummary: JSON.stringify(diff),
    });

    return {
      original,
      replayed,
      diff,
      outcome,
    };
  }

  private computeDiff(
    original: Execution,
    replayed: Execution,
    trace: ExecutionTrace,
  ): ReplayDiffSummary {
    let originalOutputs: Record<string, string> = {};
    try {
      originalOutputs = JSON.parse(original.outputs) as Record<string, string>;
    } catch {
      originalOutputs = {};
    }
    const replayOutputs = trace.nodeResults;

    const originalNodeIds = new Set(Object.keys(originalOutputs));
    const replayNodeIds = new Set(Object.keys(replayOutputs));

    const addedNodes = [...replayNodeIds].filter((id) => !originalNodeIds.has(id));
    const removedNodes = [...originalNodeIds].filter((id) => !replayNodeIds.has(id));

    const changedNodes: ReplayDiffSummary['changedNodes'] = [];
    for (const nodeId of originalNodeIds) {
      if (!replayNodeIds.has(nodeId)) continue;
      const before = originalOutputs[nodeId] ?? '';
      const after = replayOutputs[nodeId]?.output ?? '';
      if (before !== after) {
        changedNodes.push({ nodeId, originalOutput: before, replayOutput: after });
      }
    }

    return {
      addedNodes,
      removedNodes,
      changedNodes,
      totalCostDeltaUsd: replayed.costUsd - original.costUsd,
      totalLatencyDeltaMs: replayed.latencyMs - original.latencyMs,
    };
  }
}

export interface ReplayResult {
  original: Execution;
  replayed: Execution;
  diff: ReplayDiffSummary;
  outcome: 'completed' | 'diverged';
}

export class ReplayNotFoundError extends Error {
  constructor(public readonly executionId: string) {
    super(`execution ${executionId} not found or has no replayable manifest link`);
    this.name = 'ReplayNotFoundError';
  }
}

export class ReplayManifestMissingError extends Error {
  constructor(
    public readonly executionId: string,
    public readonly manifestHash: string,
  ) {
    super(`manifest ${manifestHash} referenced by execution ${executionId} is no longer in the CAS`);
    this.name = 'ReplayManifestMissingError';
  }
}

// Make the executor type structurally compatible with the real one
// without forcing the service to import Strands types.
export type { ManifestGraphExecutor };