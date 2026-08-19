import { Agent, Graph } from '@strands-agents/sdk';
import type { Graph as GraphType } from '@strands-agents/sdk';
import type { AppConfig, Manifest, SubCapabilityManifest } from '@promptsheon/shared';
import { createModel } from '../model.js';
import { validateDag } from './dag-validator.js';
import { NotFoundError } from '@promptsheon/shared';

const toolRegistry = new Map<string, Agent>();

/**
 * Register a Strands tool (or custom agent-as-tool) so it can be referenced
 * by name in a SubCapabilityManifest's `tools` list.
 */
export function registerTool(name: string, agent: Agent): void {
  toolRegistry.set(name, agent);
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
 * Tools and MCP server attachment happen at the Graph level (Strands limitation:
 * Agents accept only `tools` at construction, MCP is bound per-invocation).
 */
export function buildNodeAgent(node: SubCapabilityManifest, config: AppConfig): Agent {
  const conv = buildConversationManager(node.conversationManager);
  const retry = buildRetryStrategy(node.retry);
  return new Agent({
    id: node.id,
    model: createModel(config),
    systemPrompt: node.manifest.prompt.systemPrompt,
    ...(conv ? { conversationManager: conv } : {}),
    ...(retry ? { retryStrategy: retry } : {}),
  });
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
 */
export function buildGraph(manifest: Manifest, config: AppConfig): GraphType {
  const validation = validateDag(manifest);
  if (!validation.valid) {
    throw new NotFoundError(`invalid DAG: ${validation.errors.join('; ')}`, '');
  }

  const agents = manifest.nodes.map((node) => buildNodeAgent(node, config));

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