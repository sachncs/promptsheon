/**
 * Prompt configuration — the user-facing system and user templates.
 */
export interface PromptConfig {
  systemPrompt: string;
  userTemplate: string;
}

/**
 * Model policy — which provider/model to use and how to sample from it.
 */
export interface ModelPolicy {
  provider: string;
  modelId: string;
  temperature: number;
  maxTokens: number;
  topP?: number;
  stopSequences?: string[];
}

/**
 * Runtime policy — limits, retries, concurrency for execution.
 */
export interface RuntimePolicy {
  timeoutMs: number;
  nodeTimeoutMs: number;
  totalTimeoutMs: number;
  maxRetries: number;
  canaryPercent: number;
  concurrencyLimit: number;
}

/**
 * Context contract — structured input/output schemas and required variables.
 */
export interface ContextContract {
  inputsSchema: Record<string, unknown>;
  outputsSchema: Record<string, unknown>;
  requiredContextVars: string[];
}

/**
 * Memory configuration — stateless, session, or persistent.
 */
export interface MemoryConfig {
  enabled: boolean;
  type: 'stateless' | 'session' | 'persistent';
  retentionSec?: number;
}

/**
 * Guardrail specification — pre or post invocation check.
 */
export interface GuardrailSpec {
  type: 'regex' | 'schema' | 'llm-judge' | 'blocklist' | 'pii-redaction';
  config: Record<string, unknown>;
  enabled: boolean;
  onFailure: 'block' | 'warn' | 'redact';
}

/**
 * Tool specification — function calling tool.
 */
export interface ToolSpec {
  name: string;
  description: string;
  inputSchema: Record<string, unknown>;
  config: Record<string, unknown>;
}

/**
 * MCP server specification — external tool server.
 */
export interface McpServerSpec {
  name: string;
  url: string;
  tools: string[];
  auth?: Record<string, unknown>;
}

/**
 * Observability configuration — what to log and track per node.
 */
export interface ObservabilityConfig {
  logInputs: boolean;
  logOutputs: boolean;
  trackLatency: boolean;
  trackCost: boolean;
}

/**
 * Hook configuration — Strands lifecycle events to subscribe to.
 */
export interface HookConfig {
  beforeInvocation?: boolean;
  afterInvocation?: boolean;
  beforeModelCall?: boolean;
  afterModelCall?: boolean;
  beforeToolCall?: boolean;
  afterToolCall?: boolean;
}

/**
 * Retry specification — constant, linear, or exponential backoff.
 */
export interface RetrySpec {
  kind: 'constant' | 'linear' | 'exponential';
  maxAttempts: number;
  baseDelayMs: number;
  maxDelayMs: number;
}

/**
 * Invocation limits — turn and token caps.
 */
export interface LimitsSpec {
  turns?: number;
  outputTokens?: number;
  totalTokens?: number;
}

/**
 * State configuration — per-node state persistence.
 */
export interface StateConfig {
  enabled: boolean;
  type: 'stateless' | 'session' | 'persistent';
  retentionSec?: number;
}

/**
 * Storage configuration — agent state storage backend.
 */
export interface StorageConfig {
  kind: 'test' | 'custom';
  path?: string;
}

/**
 * Conversation manager configuration — sliding window or summarizing.
 */
export interface ConversationManagerConfig {
  kind: 'sliding-window' | 'summarizing';
  windowSize?: number;
  summaryRatio?: number;
}

/**
 * Evaluation configuration — datasets, scorers, threshold.
 */
export interface EvaluationConfig {
  datasets: string[];
  scorers: string[];
  passThreshold: number;
  primaryScorer?: string;
}

/**
 * Sub-capability manifest — a node in the DAG.
 *
 * Represents one specialised agent in a multi-agent workflow with its own
 * goal, guardrails, model config, and observability settings.
 */
export interface SubCapabilityManifest {
  id: string;
  name: string;
  description: string;
  goal: string;
  manifest: Manifest;
  dependsOn: string[];
  preGuardrails: GuardrailSpec[];
  postGuardrails: GuardrailSpec[];
  observability: ObservabilityConfig;
  hooks: HookConfig;
  retry: RetrySpec;
  conversationManager: ConversationManagerConfig;
  state: StateConfig;
  storage?: StorageConfig;
  limits: LimitsSpec;
}

/**
 * Manifest edge — directed connection between two sub-capabilities with
 * optional field mapping for output-to-input data flow.
 */
export interface ManifestEdge {
  from: string;
  to: string;
  mapping: Record<string, string>;
}

/**
 * Manifest — the complete DAG composition for a Capability.
 *
 * Immutable, content-addressed. Records the full agent definition including
 * prompt, model, runtime, context, memory, guardrails, tools, MCP servers,
 * and evaluation suite. The DAG structure (nodes + edges) defines the
 * multi-agent workflow.
 */
export interface Manifest {
  id: string;
  version: number;
  prompt: PromptConfig;
  model: ModelPolicy;
  runtime: RuntimePolicy;
  context: ContextContract;
  memory: MemoryConfig;
  guardrails: {
    pre: GuardrailSpec[];
    post: GuardrailSpec[];
  };
  tools: ToolSpec[];
  mcpServers: McpServerSpec[];
  evaluation: EvaluationConfig;
  nodes: SubCapabilityManifest[];
  edges: ManifestEdge[];
  metadata: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
}