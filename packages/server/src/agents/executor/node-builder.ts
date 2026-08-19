import { Agent, Graph, AfterInvocationEvent, BeforeInvocationEvent } from '@strands-agents/sdk';
import type { Graph as GraphType, HookCallback } from '@strands-agents/sdk';
import type { AppConfig, Manifest, SubCapabilityManifest } from '@promptsheon/shared';
import { createModel } from '../model.js';
import { validateDag } from './dag-validator.js';
import { NotFoundError } from '@promptsheon/shared';
import { createMetricsHook, type MetricsHookContext } from '../../observability/metrics-hooks.js';

const toolRegistry = new Map<string, Agent>();

/**
 * Register a Strands tool (or custom agent-as-tool) so it can be referenced
 * by name in a SubCapabilityManifest's `tools` list.
 */
export function registerTool(name: string, agent: Agent): void {
  toolRegistry.set(name, agent);
}

export interface BuildNodeAgentOptions {
  metricsHookCtx?: MetricsHookContext;
  extraHooks?: HookCallback<BeforeInvocationEvent | AfterInvocationEvent>[];
}

/**
 * Convert a SubCapabilityManifest into a fully-configured Strands Agent:
 * - model from manifest.model
 * - systemPrompt from manifest.prompt
 * - hooks from manifest.hooks
 * - retry strategy from manifest.retry
 * - conversation manager from manifest.conversationManager
 * - limits from manifest.limits
 *
 * When `metricsHookCtx` is provided, an `AfterInvocationEvent` hook is
 * registered to capture `accumulatedUsage` tokens and persist them as
 * a `node_runs` row.
 *
 * Tools and MCP server attachment happen at the Graph level (Strands limitation:
 * Agents accept only `tools` at construction, MCP is bound per-invocation).
 */
export function buildNodeAgent(
  node: SubCapabilityManifest,
  config: AppConfig,
  options: BuildNodeAgentOptions = {},
): Agent {
  const conv = buildConversationManager(node.conversationManager);
  const retry = buildRetryStrategy(node.retry);
  const agent = new Agent({
    id: node.id,
    model: createModel(config),
    systemPrompt: node.manifest.prompt.systemPrompt,
    ...(conv ? { conversationManager: conv } : {}),
    ...(retry ? { retryStrategy: retry } : {}),
  });

  if (options.metricsHookCtx) {
    const cb = createMetricsHook(options.metricsHookCtx);
    agent.addHook(AfterInvocationEvent, cb);
  }
  for (const cb of options.extraHooks ?? []) {
    if (cb.length <= 1) {
      agent.addHook(AfterInvocationEvent, cb as HookCallback<AfterInvocationEvent>);
    } else {
      agent.addHook(BeforeInvocationEvent, cb as HookCallback<BeforeInvocationEvent>);
    }
  }

  return agent;
}

export function buildInvocationLimits(config: { turns?: number; outputTokens?: number; totalTokens?: number }): { turns?: number; outputTokens?: number; totalTokens?: number } | undefined {
  if (!config.turns && !config.outputTokens && !config.totalTokens) return undefined;
  return {
    ...(config.turns ? { turns: config.turns } : {}),
    ...(config.outputTokens ? { outputTokens: config.outputTokens } : {}),
    ...(config.totalTokens ? { totalTokens: config.totalTokens } : {}),
  };
}

function buildConversationManager(config: { kind: 'sliding-window' | 'summarizing'; windowSize?: number }) {
  if (config.kind === 'sliding-window') {
    return new (require('@strands-agents/sdk').SlidingWindowConversationManager)({
      windowSize: config.windowSize ?? 20,
    });
  }
  return undefined;
}

function buildRetryStrategy(config: { kind: 'constant' | 'linear' | 'exponential'; maxAttempts: number; baseDelayMs: number; maxDelayMs: number }) {
  const { ConstantBackoff, LinearBackoff, ExponentialBackoff } = require('@strands-agents/sdk');
  switch (config.kind) {
    case 'constant':
      return new ConstantBackoff({ maxAttempts: config.maxAttempts, baseDelayMs: config.baseDelayMs, maxDelayMs: config.baseDelayMs });
    case 'linear':
      return new LinearBackoff({ maxAttempts: config.maxAttempts, baseDelayMs: config.baseDelayMs, maxDelayMs: config.maxDelayMs });
    case 'exponential':
    default:
      return new ExponentialBackoff({ maxAttempts: config.maxAttempts, baseDelayMs: config.baseDelayMs, maxDelayMs: config.maxDelayMs });
  }
}

/**
 * Build a Strands Graph from a Manifest DAG.
 *
 * Throws NotFoundError if the DAG is invalid (caught by global error handler
 * to return 404/422 depending on context).
 *
 * When `metricsHookCtx` is provided, every node agent gets the metrics
 * persistence hook registered (see `buildNodeAgent`).
 */
export function buildGraph(
  manifest: Manifest,
  config: AppConfig,
  options: BuildNodeAgentOptions = {},
): GraphType {
  const validation = validateDag(manifest);
  if (!validation.valid) {
    throw new NotFoundError(`invalid DAG: ${validation.errors.join('; ')}`, '');
  }

  const agents = manifest.nodes.map((node) => buildNodeAgent(node, config, options));

  const edges: [string, string][] = manifest.edges.map((e) => [e.from, e.to]);

  const sources = findRootNodes(manifest);

  return new Graph({
    nodes: agents,
    edges,
    sources,
    maxSteps: Math.max(manifest.nodes.length * 2, 10),
    timeout: manifest.runtime.totalTimeoutMs,
    nodeTimeout: manifest.runtime.nodeTimeoutMs,
  });
}

function findRootNodes(manifest: Manifest): string[] {
  const incoming = new Set<string>();
  for (const edge of manifest.edges) {
    incoming.add(edge.to);
  }
  return manifest.nodes.filter((n) => !incoming.has(n.id)).map((n) => n.id);
}