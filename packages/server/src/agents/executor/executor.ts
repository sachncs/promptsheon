import type { AppConfig, Manifest } from '@promptsheon/shared';
import { Agent } from '@strands-agents/sdk';
import { SseHub } from '../../sse/hub.js';
import { buildGraph, buildInvocationLimits, buildNodeAgent } from './node-builder.js';
import { validateDag } from './dag-validator.js';
import { runAllGuardrails, type GuardrailBroadcast } from './guardrails.js';
import { NotFoundError } from '@promptsheon/shared';
import type { ManifestRepo } from '../../repos/manifest.js';
import { ChaosConfig, ChaosFailureError } from '../../hardening/chaos.js';
import { checkCostCap, recordCost, type CostLimitConfig } from '../../hardening/cost-caps.js';
import { findRedTeamMatches } from '../../hardening/redteam.js';

export interface ExecutionTrace {
  executionId: string;
  manifestHash: string;
  status: 'completed' | 'failed' | 'cancelled';
  startedAt: string;
  endedAt: string;
  nodeResults: Record<string, NodeRunResult>;
  totalCost: number;
  totalLatencyMs: number;
  totalTokens: number;
  error?: string;
}

export interface NodeRunResult {
  nodeId: string;
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';
  output: string;
  latencyMs: number;
  costUsd: number;
  totalTokens: number;
  error: string;
}

export interface ExecuteOptions {
  executionId: string;
  inputs: Record<string, unknown>;
  userId?: string;
  environment?: string;
  traceId?: string;
  signal?: AbortSignal;
}

/**
 * ManifestGraphExecutor — Strands Graph-based executor for a Manifest DAG.
 *
 * Per-node execution:
 * 1. Pre-guardrails run on inputs (block/warn/redact per spec.onFailure)
 * 2. Agent.invoke() with manifest.prompt + node.manifest.prompt + inputs
 * 3. Post-guardrails run on outputs
 * 4. NodeRunResult accumulated into ExecutionTrace
 *
 * Hooks:
 * - BeforeInvocationEvent runs all pre-guardrails
 * - AfterModelCallEvent runs all post-guardrails
 * - BeforeNodeCallEvent emits SSE start event
 * - AfterNodeCallEvent emits SSE complete event
 *
 * Streaming: emits one SSE event per node lifecycle transition.
 * Cancellation: caller passes AbortSignal via options.signal; loop checks
 * between nodes.
 */
export class ManifestGraphExecutor {
  /**
   * Process-local registry of agents currently in flight, keyed by
   * `<executionId>:<nodeId>`. Populated by `execute()` and cleared
   * when the execution finishes. Allows `POST /api/snapshots` to
   * resolve a running agent for `takeSnapshot` / `loadSnapshot`.
   */
  private readonly liveAgents = new Map<string, Agent>();

  constructor(
    private deps: {
      config: AppConfig;
      hub: SseHub;
      manifestRepo?: ManifestRepo;
      chaos?: ChaosConfig;
      costCap?: CostLimitConfig;
      costOrgId?: string;
      costCapabilityId?: string;
    },
  ) {}

  async execute(manifestHash: string, manifest: Manifest, options: ExecuteOptions): Promise<ExecutionTrace> {
    const validation = validateDag(manifest);
    if (!validation.valid) {
      throw new NotFoundError(`invalid DAG: ${validation.errors.join('; ')}`, '');
    }

    const executionStartedAtMs = Date.now();
    const metricsHookCtx = this.deps.manifestRepo
      ? { executionId: options.executionId, manifestHash, manifestRepo: this.deps.manifestRepo }
      : undefined;
    // Validate DAG via buildGraph (which constructs a Strands Graph
    // and runs validateDag() during construction). The Graph is built
    // for type/dag validation; per-node execution happens in the
    // loop below so the domain-specific guardrail/cost-cap/chaos/
    // metrics/SSE-event semantics can be applied between Strands
    // calls — a single Strands Graph.invoke() would drop all of
    // these observability + safety surfaces.
    buildGraph(manifest, this.deps.config, metricsHookCtx ? { metricsHookCtx } : {});
    const startedAt = new Date().toISOString();
    const broadcast: GuardrailBroadcast = { hub: this.deps.hub, config: this.deps.config };

    const trace: ExecutionTrace = {
      executionId: options.executionId,
      manifestHash,
      status: 'completed',
      startedAt,
      endedAt: '',
      nodeResults: {},
      totalCost: 0,
      totalLatencyMs: 0,
      totalTokens: 0,
    };

    this.deps.hub.broadcast({
      type: 'status',
      data: { kind: 'execution_start', executionId: options.executionId, manifestHash, manifestName: manifest.id, startedAt },
      timestamp: startedAt,
    });

    for (const node of manifest.nodes) {
      if (options.signal?.aborted) {
        trace.status = 'cancelled';
        break;
      }
      const nodeStartedAt = Date.now();
      trace.nodeResults[node.id] = {
        nodeId: node.id,
        status: 'running',
        output: '',
        latencyMs: 0,
        costUsd: 0,
        totalTokens: 0,
        error: '',
      };
      this.deps.hub.broadcast({
        type: 'status',
        data: { kind: 'node_start', executionId: options.executionId, nodeId: node.id },
        timestamp: new Date().toISOString(),
      });

      const preCheck = runAllGuardrails(node.preGuardrails, {
        manifest,
        nodeId: node.id,
        executionId: options.executionId,
        values: [JSON.stringify(options.inputs)],
        phase: 'pre',
      }, broadcast);

      // Red-team scan: any of the configured patterns matching an
      // input value fails the node with a 4xx-class error. The
      // pattern set is global (config-driven) — see `hardening/redteam.ts`.
      if (this.deps.costCap) {
        const redteamHits = findRedTeamMatches(JSON.stringify(options.inputs));
        if (redteamHits.length > 0) {
          trace.nodeResults[node.id]!.status = 'failed';
          trace.nodeResults[node.id]!.error = `red-team: ${redteamHits.join(', ')}`;
          trace.status = 'failed';
          trace.error = `red-team pattern matched on node ${node.id}: ${redteamHits.join(', ')}`;
          this.deps.hub.broadcast({
            type: 'error',
            data: { kind: 'redteam_blocked', executionId: options.executionId, nodeId: node.id, patterns: redteamHits },
            timestamp: new Date().toISOString(),
          });
          break;
        }
      }

      // Pre-invocation cost-cap check. Per-org / per-capability / per-invocation.
      if (this.deps.costCap) {
        const estimatedTokens = JSON.stringify(options.inputs).length / 4;
        const estimatedCost = (estimatedTokens / 1000) * 0.00003;
        const capResult = checkCostCap(
          {
            orgId: this.deps.costOrgId ?? 'unknown',
            capabilityId: this.deps.costCapabilityId ?? manifest.metadata['capabilityId'] as string ?? 'unknown',
            estimatedCostUsd: estimatedCost,
            config: this.deps.costCap,
          },
          { allowFailover: false },
        );
        if (!capResult.allowed) {
          trace.nodeResults[node.id]!.status = 'failed';
          trace.nodeResults[node.id]!.error = `cost-cap: ${capResult.reason}`;
          trace.status = 'failed';
          trace.error = `cost cap exceeded on node ${node.id}: ${capResult.reason}`;
          this.deps.hub.broadcast({
            type: 'error',
            data: { kind: 'cost_cap_blocked', executionId: options.executionId, nodeId: node.id, reason: capResult.reason },
            timestamp: new Date().toISOString(),
          });
          break;
        }
      }

      if (!preCheck.allowed) {
        trace.nodeResults[node.id]!.status = 'failed';
        trace.nodeResults[node.id]!.error = 'pre-guardrail blocked';
        trace.status = 'failed';
        trace.error = `pre-guardrail blocked for node ${node.id}`;
        break;
      }

      try {
        const chaosFailure = this.deps.chaos?.shouldFail(node.id);
        if (chaosFailure) {
          if (chaosFailure.kind === 'timeout' && chaosFailure.delayMs) {
            await new Promise((resolve) => setTimeout(resolve, chaosFailure.delayMs));
          }
          throw new ChaosFailureError(node.id, chaosFailure);
        }

        const perNodeHookCtx = metricsHookCtx
          ? { ...metricsHookCtx, invocationStartedAt: Date.now() }
          : undefined;
        const agent = buildNodeAgent(node, this.deps.config, perNodeHookCtx ? { metricsHookCtx: perNodeHookCtx } : {});
        const agentKey = `${options.executionId}:${node.id}`;
        this.liveAgents.set(agentKey, agent);
        const limits = buildInvocationLimits(node.limits);
        const result = await agent.invoke(
          this.buildPrompt(node, options.inputs, preCheck.redactedValues[0] as string | undefined),
          { ...(limits ? { limits } : {}) },
        );
        this.liveAgents.delete(agentKey);
        const outputText = this.extractText(result);
        const metrics = result.metrics;
        const totalTokens = metrics?.accumulatedUsage?.totalTokens ?? 0;

        const postCheck = runAllGuardrails(node.postGuardrails, {
          manifest,
          nodeId: node.id,
          executionId: options.executionId,
          values: [outputText],
          phase: 'post',
        }, broadcast);

        const finalOutput = postCheck.redactedValues[0] as string ?? outputText;
        const latencyMs = Date.now() - nodeStartedAt;
        const cost = metrics?.accumulatedUsage?.totalTokens
          ? (metrics.accumulatedUsage.totalTokens / 1000) * 0.00003
          : 0;

        trace.nodeResults[node.id] = {
          nodeId: node.id,
          status: 'completed',
          output: finalOutput,
          latencyMs,
          costUsd: cost,
          totalTokens,
          error: '',
        };
        trace.totalCost += cost;
        trace.totalLatencyMs += latencyMs;
        trace.totalTokens += totalTokens;

        if (this.deps.costCap) {
          recordCost(
            this.deps.costOrgId ?? 'unknown',
            this.deps.costCapabilityId ?? manifest.metadata['capabilityId'] as string ?? 'unknown',
            cost,
            this.deps.costCap,
          );
        }

        this.deps.hub.broadcast({
          type: 'status',
          data: { kind: 'node_complete', executionId: options.executionId, nodeId: node.id, latencyMs, totalTokens },
          timestamp: new Date().toISOString(),
        });
      } catch (e) {
        const err = e as Error;
        const latencyMs = Date.now() - nodeStartedAt;
        trace.nodeResults[node.id]!.status = 'failed';
        trace.nodeResults[node.id]!.error = err.message;
        trace.nodeResults[node.id]!.latencyMs = latencyMs;
        trace.status = 'failed';
        trace.error = `node ${node.id} failed: ${err.message}`;
        this.deps.hub.broadcast({
          type: 'error',
          data: { kind: 'node_failed', executionId: options.executionId, nodeId: node.id, error: err.message },
          timestamp: new Date().toISOString(),
        });
        break;
      }
    }

    trace.endedAt = new Date().toISOString();
    this.deps.hub.broadcast({
      type: 'complete',
      data: { kind: 'execution_complete', executionId: options.executionId, status: trace.status, totalCost: trace.totalCost, totalLatencyMs: trace.totalLatencyMs, endedAt: trace.endedAt },
      timestamp: trace.endedAt,
    });
    return trace;
  }

  /**
   * Resolve a live agent by `executionId:nodeId` for snapshot/restore.
   * Returns null if the agent is not currently in flight.
   */
  getLiveAgent(executionId: string, nodeId: string): Agent | null {
    return this.liveAgents.get(`${executionId}:${nodeId}`) ?? null;
  }

  private buildPrompt(node: Manifest['nodes'][number], inputs: Record<string, unknown>, redactedJson: string | undefined): string {
    const userTemplate = node.manifest.prompt.userTemplate || '{{input}}';
    const filled = userTemplate.replace(/\{\{input\}\}/g, redactedJson ?? JSON.stringify(inputs));
    return `${node.manifest.prompt.systemPrompt}\n\n# User input\n${filled}`;
  }

  private extractText(result: unknown): string {
    const r = result as { lastMessage?: { content?: Array<{ type: string; text?: string }> } };
    if (!r.lastMessage?.content) return '';
    return r.lastMessage.content
      .filter((b) => b.type === 'textBlock')
      .map((b) => b.text ?? '')
      .join('');
  }
}