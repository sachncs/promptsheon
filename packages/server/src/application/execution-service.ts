import type { Manifest } from '@promptsheon/shared';
import { createHash } from 'node:crypto';

/** The trace shape persisted and returned by an execution run. */
export interface ExecutionTrace {
  executionId: string;
  manifestHash: string;
  status: 'completed' | 'failed' | 'cancelled';
  startedAt: string;
  endedAt: string;
  nodeResults: Record<string, { output: string }>;
  totalCost: number;
  totalLatencyMs: number;
  totalTokens: number;
  error?: string;
}

/** Options supplied by the HTTP adapter to an execution run. */
export interface ExecutionRunOptions {
  executionId: string;
  inputs: Record<string, unknown>;
  environment: string;
  traceId?: string;
  signal?: AbortSignal;
}

/** Port for loading an organization-owned manifest. */
export interface ExecutionManifestStore {
  findByHashInOrg(hash: string, organizationId: string): Manifest | null;
}

/** Port for selecting active releases for an organization-owned manifest. */
export interface ExecutionReleaseStore {
  findActiveByManifestHashInOrg(
    manifestHash: string,
    organizationId: string,
  ): Array<{ id: string; canaryPercent: number }>;
}

/** Port for recording trace lifecycle events. */
export interface ExecutionTraceStore {
  startRun(input: {
    organizationId: string;
    executionId: string;
    environment: string;
    name: string;
    model: string | null;
    attributes: Record<string, unknown>;
  }): { id: string };
  finalize(id: string, status: 'success' | 'error', totals: { tokens: number; costUsd: number }): void;
}

/** Port for running a manifest without coupling the use case to an agent SDK. */
export interface ExecutionRunner {
  execute(
    manifestHash: string,
    manifest: Manifest,
    options: ExecutionRunOptions & { traceRunId: string },
  ): Promise<ExecutionTrace>;
}

/** Input accepted by the execution persistence adapter. */
export interface ExecutionRecord {
  capabilityVersionId: string | null;
  inputs: string;
  inputHash: string;
  outputs: string;
  model: string;
  provider: string;
  latencyMs: number;
  costUsd: number;
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
  error: string;
  traceId: string;
  environment: string;
}

/** Port for persisting the completed execution record. */
export interface ExecutionRecordStore {
  create(record: ExecutionRecord): unknown;
}

/** Result of the application execution use case. */
export type ExecutionRunResult =
  | { kind: 'manifest-not-found' }
  | { kind: 'no-active-release' }
  | { kind: 'success'; trace: ExecutionTrace; pickedReleaseId: string };

/** Coordinates manifest lookup, canary selection, execution, tracing, and persistence. */
export class ExecutionService {
  constructor(
    private readonly manifests: ExecutionManifestStore,
    private readonly releases: ExecutionReleaseStore,
    private readonly traces: ExecutionTraceStore,
    private readonly records: ExecutionRecordStore,
    private readonly runner: ExecutionRunner,
    private readonly selectRelease: (pool: Array<{ id: string; canaryPercent: number }>) => string | null,
  ) {}

  async run(
    manifestHash: string,
    organizationId: string,
    options: ExecutionRunOptions,
  ): Promise<ExecutionRunResult> {
    const manifest = this.manifests.findByHashInOrg(manifestHash, organizationId);
    if (!manifest) return { kind: 'manifest-not-found' };

    const activeReleases = this.releases.findActiveByManifestHashInOrg(manifestHash, organizationId);
    const pickedReleaseId = this.selectRelease(activeReleases);
    if (!pickedReleaseId) return { kind: 'no-active-release' };

    const traceRun = this.traces.startRun({
      organizationId,
      executionId: options.executionId,
      environment: options.environment,
      name: `manifest:${manifestHash.slice(0, 12)}`,
      model: manifest.model?.modelId ?? null,
      attributes: { manifestHash, route: '/api/executions' },
    });

    let trace: ExecutionTrace;
    try {
      trace = await this.runner.execute(manifestHash, manifest, { ...options, traceRunId: traceRun.id });
    } catch (error) {
      this.traces.finalize(traceRun.id, 'error', { tokens: 0, costUsd: 0 });
      throw error;
    }

    this.traces.finalize(traceRun.id, trace.status === 'completed' ? 'success' : 'error', {
      tokens: trace.totalTokens,
      costUsd: trace.totalCost,
    });
    this.records.create({
      capabilityVersionId: manifest.id,
      inputs: JSON.stringify(options.inputs),
      inputHash: hashInputs(options.inputs),
      outputs: JSON.stringify(trace.nodeResults),
      model: manifest.model.modelId,
      provider: manifest.model.provider,
      latencyMs: trace.totalLatencyMs,
      costUsd: trace.totalCost,
      promptTokens: 0,
      completionTokens: 0,
      totalTokens: trace.totalTokens,
      error: trace.error ?? '',
      traceId: options.traceId ?? options.executionId,
      environment: options.environment,
    });
    return { kind: 'success', trace, pickedReleaseId };
  }
}

function hashInputs(inputs: Record<string, unknown>): string {
  return createHash('sha256').update(JSON.stringify(inputs)).digest('hex');
}
