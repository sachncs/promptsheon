import { createHash, randomUUID } from 'node:crypto';
import type { Manifest } from '@promptsheon/shared';
import type { AgentSpecificationRecord } from '../repos/agent-specification.js';
import type { ExecutionJob, ExecutionJobRepo, ExecutionQueueMetrics } from '../repos/execution-job.js';
import { DurableExecutionWorker, type ExecutionWorkerOptions } from './durable-execution-worker.js';
import { z } from 'zod';
import type { ExecutionCheckpointRepo } from '../repos/execution-checkpoint.js';
import { ExecutionTimeoutError } from '../agents/executor/executor.js';
import { ExecutionWorkError } from './durable-execution-worker.js';
import type { ToolAuthorizer, ToolRegistry } from './execution-ports.js';
import type { EvidenceRecorder } from '../observability/evidence-sink.js';
import type { TraceRepo } from '../repos/trace.js';

export class ExecutionWorkspaceScopeError extends Error {
  constructor(jobId: string, workspaceId: string) {
    super(`execution job ${jobId} is not in workspace ${workspaceId}`);
    this.name = 'ExecutionWorkspaceScopeError';
  }
}

interface ManifestStore {
  get(workspaceId: string, hash: string): Promise<AgentSpecificationRecord>;
}

interface ManifestRunner {
  execute(hash: string, manifest: Manifest, options: {
    executionId: string;
    inputs: Record<string, unknown>;
    organizationId?: string;
    workspaceId?: string;
    toolRegistry?: ToolRegistry;
    toolAuthorizer?: ToolAuthorizer;
    traceRunId?: string;
    signal?: AbortSignal;
    checkpoints?: {
      list(executionId: string): Promise<Array<{ stepId: string; state: 'completed' | 'failed'; output: string }>>;
      save(input: { executionId: string; stepId: string; state: 'completed' | 'failed'; output: string; metadata: Record<string, unknown> }): Promise<unknown>;
    };
  }): Promise<unknown>;
}

/** Application boundary for durable execution submission and worker lifecycle. */
export class DurableExecutionService {
  private worker: DurableExecutionWorker | undefined;
  constructor(
    private readonly jobs: ExecutionJobRepo,
    private readonly manifests: ManifestStore,
    private readonly runner: ManifestRunner,
    private readonly checkpoints: ExecutionCheckpointRepo,
    private readonly tools?: ToolRegistry,
    private readonly toolAuthorizer?: ToolAuthorizer,
    private readonly evidence?: EvidenceRecorder,
    private readonly traces?: TraceRepo,
  ) {}

  enqueue(input: {
    organizationId: string;
    workspaceId: string;
    agentHash: string;
    inputs: Record<string, unknown>;
    idempotencyKey: string;
    maxAttempts?: number;
  }): ExecutionJob {
    const inputJson = stableJson(input.inputs);
    const inputHash = createHash('sha256').update(inputJson, 'utf8').digest('hex');
    return this.jobs.enqueue({
      organizationId: input.organizationId,
      workspaceId: input.workspaceId,
      agentHash: input.agentHash,
      inputHash,
      inputJson,
      idempotencyKey: input.idempotencyKey,
      ...(input.maxAttempts === undefined ? {} : { maxAttempts: input.maxAttempts }),
    });
  }

  get(organizationId: string, workspaceId: string, id: string): ExecutionJob {
    const job = this.jobs.get(organizationId, id);
    if (job.workspaceId !== workspaceId) throw new ExecutionWorkspaceScopeError(id, workspaceId);
    return job;
  }

  metrics(organizationId: string): ExecutionQueueMetrics {
    return this.jobs.metrics(organizationId);
  }

  cancel(organizationId: string, workspaceId: string, id: string): ExecutionJob {
    const job = this.jobs.get(organizationId, id);
    if (job.workspaceId !== workspaceId) throw new ExecutionWorkspaceScopeError(id, workspaceId);
    if (this.worker) return this.worker.cancel(organizationId, id);
    return this.jobs.cancel(organizationId, id);
  }

  createWorker(options: Partial<ExecutionWorkerOptions> = {}): DurableExecutionWorker {
    this.worker = new DurableExecutionWorker(this.jobs, {
      run: async (job, context) => {
        const trace = this.traces?.startRun({
          organizationId: job.organizationId,
          name: `execution:${job.id}`,
          attributes: { executionId: job.id, agentHash: job.agentHash, workspaceId: job.workspaceId },
        });
        const traceRunId = trace?.id;
        this.recordEvidence({
          eventType: 'execution.started',
          job,
          payload: { inputHash: job.inputHash, attempt: job.attempts },
          traceRunId,
        });
        try {
          const record = await this.manifests.get(job.workspaceId, job.agentHash);
          const inputs = z.record(z.string(), z.unknown()).parse(JSON.parse(job.inputJson));
          const estimatedInputTokens = Math.ceil(job.inputJson.length / 4);
          if (estimatedInputTokens > record.specification.resourceBudget.maxInputTokens) {
            throw new ExecutionWorkError('input token budget exhausted');
          }
          const manifest = toManifest(record);
          const allowedTools = new Set(
            Array.isArray(manifest.metadata['allowedTools'])
              ? manifest.metadata['allowedTools'].filter((tool): tool is string => typeof tool === 'string')
              : [],
          );
          const toolAuthorizer: ToolAuthorizer = {
            authorize: async (toolName, organizationId, executionId) => {
              if (organizationId !== job.organizationId || executionId !== job.id) return false;
              if (!allowedTools.has(toolName)) {
                this.recordEvidence({ eventType: 'permission.decided', job, payload: { toolName, allowed: false } });
                return false;
              }
              const allowed = this.toolAuthorizer?.authorize(toolName, organizationId, executionId) ?? true;
              const result = await allowed;
              this.recordEvidence({ eventType: 'permission.decided', job, payload: { toolName, allowed: result } });
              return result;
            },
          };
          const result = await this.runner.execute(job.agentHash, manifest, {
            executionId: job.id,
            inputs,
            organizationId: job.organizationId,
            workspaceId: job.workspaceId,
            ...(traceRunId ? { traceRunId } : {}),
            ...(this.tools ? { toolRegistry: this.tools } : {}),
            ...(this.tools ? { toolAuthorizer } : {}),
            signal: context.signal,
            checkpoints: context.checkpoint,
          });
          const totals = executionTotals(result);
          if (totals.totalTokens > record.specification.resourceBudget.maxInputTokens + record.specification.resourceBudget.maxOutputTokens) {
            throw new ExecutionWorkError('token budget exhausted');
          }
          if (totals.costUsd > record.specification.resourceBudget.maxCostUsd) {
            throw new ExecutionWorkError('cost budget exhausted');
          }
          this.recordEvidence({
            eventType: 'execution.completed',
            job,
            payload: totals,
            traceRunId,
          });
          this.recordEvidence({
            eventType: 'resource.consumed',
            job,
            payload: totals,
            traceRunId,
          });
          // The queue-backed sink is intentionally asynchronous during the
          // execution, but the terminal job state must not become visible
          // before its evidence is durable. Otherwise clients can observe a
          // completed job with only a partial evidence timeline.
          await this.evidence?.flush();
          if (traceRunId) this.traces?.finalize(traceRunId, 'success', { tokens: totals.totalTokens, costUsd: totals.costUsd });
          return result;
        } catch (error) {
          this.recordEvidence({
            eventType: context.signal.aborted ? 'execution.cancelled' : 'execution.failed',
            job,
            payload: { error: error instanceof Error ? error.message : String(error) },
            traceRunId,
          });
          this.recordEvidence({
            eventType: 'error.observed',
            job,
            payload: { error: error instanceof Error ? error.name : 'unknown' },
            traceRunId,
          });
          await this.evidence?.flush();
          if (traceRunId) this.traces?.finalize(traceRunId, 'error');
          if (error instanceof ExecutionTimeoutError) throw new ExecutionWorkError(error.message, false, true);
          throw error;
        }
      },
    }, {
      workerId: options.workerId ?? `worker-${randomUUID()}`,
      maxConcurrency: options.maxConcurrency ?? 4,
      pollMs: options.pollMs ?? 100,
      leaseMs: options.leaseMs ?? 300_000,
      maxBackoffMs: options.maxBackoffMs ?? 30_000,
      maxConcurrencyPerOrganization: options.maxConcurrencyPerOrganization ?? 2,
      ...(options.random === undefined ? {} : { random: options.random }),
    }, this.checkpoints);
    return this.worker;
  }

  private recordEvidence(input: {
    eventType: 'execution.started' | 'execution.completed' | 'execution.failed' | 'execution.cancelled' | 'permission.decided' | 'resource.consumed' | 'error.observed';
    job: ExecutionJob;
    payload: unknown;
    traceRunId?: string;
  }): void {
    if (!this.evidence) return;
    try {
      this.evidence.record({
        eventType: input.eventType,
        organizationId: input.job.organizationId,
        workspaceId: input.job.workspaceId,
        correlationId: input.job.id,
        ...(input.traceRunId ? { traceId: input.traceRunId } : {}),
        executionId: input.job.id,
        agentHash: input.job.agentHash,
        payload: input.payload,
      });
    } catch {
      // Evidence is non-critical telemetry. Execution state remains authoritative.
    }
  }
}

function stableJson(value: unknown): string {
  if (value === undefined) return 'null';
  if (value === null || typeof value !== 'object') return JSON.stringify(value);
  if (Array.isArray(value)) return `[${value.map(stableJson).join(',')}]`;
  const record = value as Record<string, unknown>;
  return `{${Object.keys(record).sort().map((key) => `${JSON.stringify(key)}:${stableJson(record[key])}`).join(',')}}`;
}

function executionTotals(value: unknown): { totalTokens: number; costUsd: number; totalLatencyMs: number } {
  if (!value || typeof value !== 'object') return { totalTokens: 0, costUsd: 0, totalLatencyMs: 0 };
  const record = value as Record<string, unknown>;
  return {
    totalTokens: typeof record['totalTokens'] === 'number' ? record['totalTokens'] : 0,
    costUsd: typeof record['totalCost'] === 'number' ? record['totalCost'] : 0,
    totalLatencyMs: typeof record['totalLatencyMs'] === 'number' ? record['totalLatencyMs'] : 0,
  };
}

function toManifest(record: AgentSpecificationRecord): Manifest {
  const specification = record.specification;
  const guardrails = specification.guardrails.map((guardrail) => ({
    type: 'blocklist' as const,
    config: { rule: guardrail.rule, ...guardrail.config },
    enabled: true,
    onFailure: guardrail.failureMode,
  }));
  const leaf: Manifest = {
    id: `${record.hash}:leaf`,
    version: 1,
    prompt: {
      systemPrompt: [specification.prompt.system, specification.prompt.developer].filter(Boolean).join('\n\n'),
      userTemplate: specification.prompt.template,
    },
    model: {
      provider: specification.modelPolicy.provider,
      modelId: specification.modelPolicy.model,
      temperature: specification.modelPolicy.temperature,
      maxTokens: specification.modelPolicy.maxOutputTokens,
      topP: specification.modelPolicy.topP,
    },
    runtime: {
      timeoutMs: specification.executionPolicy.timeoutMs,
      nodeTimeoutMs: specification.executionPolicy.timeoutMs,
      totalTimeoutMs: specification.resourceBudget.maxWallTimeMs,
      maxRetries: Math.max(0, specification.executionPolicy.maxAttempts - 1),
      canaryPercent: 0,
      concurrencyLimit: specification.executionPolicy.concurrency,
    },
    context: { inputsSchema: {}, outputsSchema: {}, requiredContextVars: specification.contextPolicy.required },
    memory: {
      enabled: specification.memoryPolicy.mode !== 'none',
      type: specification.memoryPolicy.mode === 'persistent' ? 'persistent' : specification.memoryPolicy.mode === 'session' ? 'session' : 'stateless',
      retentionSec: specification.memoryPolicy.retentionSeconds || undefined,
    },
    guardrails: { pre: guardrails.filter((_, index) => specification.guardrails[index]?.kind === 'input'), post: guardrails.filter((_, index) => specification.guardrails[index]?.kind === 'output') },
    tools: specification.capabilities.tools.map((tool) => ({ name: tool.name, description: tool.description, inputSchema: tool.inputSchema, config: {} })),
    mcpServers: [],
    evaluation: { datasets: specification.evaluationPolicy.suites, scorers: [], passThreshold: specification.evaluationPolicy.requiredScore },
    nodes: [],
    edges: [],
    metadata: { agentHash: record.hash, workspaceId: record.workspaceId, allowedTools: specification.permissions.allowedTools },
    createdAt: record.createdAt,
    updatedAt: record.createdAt,
  };
  return {
    ...leaf,
    nodes: [{
      id: 'root', name: specification.role, description: specification.objective, goal: specification.objective,
      manifest: leaf, dependsOn: [], preGuardrails: leaf.guardrails.pre, postGuardrails: leaf.guardrails.post,
      observability: { logInputs: true, logOutputs: true, trackLatency: true, trackCost: true },
      hooks: {}, retry: { kind: 'exponential', maxAttempts: specification.executionPolicy.maxAttempts, baseDelayMs: 100, maxDelayMs: 10_000 },
      conversationManager: { kind: 'sliding-window', windowSize: 20 }, state: { enabled: false, type: 'stateless' }, limits: { outputTokens: specification.resourceBudget.maxOutputTokens, totalTokens: specification.resourceBudget.maxInputTokens + specification.resourceBudget.maxOutputTokens },
    }],
  };
}
