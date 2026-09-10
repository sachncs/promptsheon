import type { Manifest, SubCapabilityManifest } from './types/manifest.js';
import {
  ManifestSchema,
  SubCapabilityManifestSchema,
  ManifestEdgeSchema,
  GuardrailSpecSchema,
} from './validation.js';
import { z } from 'zod';

const GUARDRAIL_TYPES = ['regex', 'schema', 'llm-judge', 'blocklist', 'pii-redaction'] as const;
const GUARDRAIL_FAILURE_MODES = ['block', 'warn', 'redact'] as const;
const RETRY_KINDS = ['constant', 'linear', 'exponential'] as const;
const MEMORY_TYPES = ['stateless', 'session', 'persistent'] as const;
const CONVERSATION_MANAGER_KINDS = ['sliding-window', 'summarizing'] as const;
const STORAGE_KINDS = ['test', 'custom'] as const;

export const GuardrailTypeSchema = z.enum(GUARDRAIL_TYPES);
export const ManifestGuardrailFailureModeSchema = z.enum(GUARDRAIL_FAILURE_MODES);

export type BuildManifestOverrides = {
  id?: string;
  version?: number;
  prompt?: Manifest['prompt'];
  model?: Partial<Manifest['model']>;
  runtime?: Partial<Manifest['runtime']>;
  context?: Partial<Manifest['context']>;
  memory?: Partial<Manifest['memory']>;
  guardrails?: Partial<Manifest['guardrails']>;
  tools?: Manifest['tools'];
  mcpServers?: Manifest['mcpServers'];
  evaluation?: Partial<Manifest['evaluation']>;
  nodes?: Array<Partial<SubCapabilityManifest> & { id: string; goal: string; manifest: unknown }>;
  edges?: Manifest['edges'];
  metadata?: Record<string, unknown>;
};

export function buildValidManifest(overrides: BuildManifestOverrides = {}): Record<string, unknown> {
  const model = {
    provider: 'openai',
    modelId: 'gpt-4',
    temperature: 0.7,
    maxTokens: 4096,
    ...overrides.model,
  };
  const runtime = {
    timeoutMs: 30000,
    nodeTimeoutMs: 10000,
    totalTimeoutMs: 300000,
    maxRetries: 3,
    canaryPercent: 0,
    concurrencyLimit: 10,
    ...overrides.runtime,
  };
  const context = {
    inputsSchema: {},
    outputsSchema: {},
    requiredContextVars: [],
    ...overrides.context,
  };
  const memory = {
    enabled: false,
    type: 'stateless' as const,
    ...overrides.memory,
  };
  const guardrails = {
    pre: [],
    post: [],
    ...overrides.guardrails,
  };
  const evaluation = {
    datasets: [],
    scorers: [],
    passThreshold: 0.7,
    primaryScorer: 'goal-success-rate',
    ...overrides.evaluation,
  };

  const manifest: Record<string, unknown> = {
    id: overrides.id ?? 'manifest-1',
    version: overrides.version ?? 1,
    prompt: overrides.prompt ?? {
      systemPrompt: 'You are a helpful assistant.',
      userTemplate: '{{input}}',
    },
    model,
    runtime,
    context,
    memory,
    guardrails,
    tools: overrides.tools ?? [],
    mcpServers: overrides.mcpServers ?? [],
    evaluation,
    nodes: (overrides.nodes ?? []).map((n) => ({
      id: n.id,
      name: n.name ?? `node-${n.id}`,
      description: n.description ?? '',
      goal: n.goal,
      manifest: n.manifest,
      dependsOn: n.dependsOn ?? [],
      preGuardrails: n.preGuardrails ?? [],
      postGuardrails: n.postGuardrails ?? [],
      observability: n.observability ?? {
        logInputs: true,
        logOutputs: true,
        trackLatency: true,
        trackCost: true,
      },
      hooks: n.hooks ?? {
        beforeInvocation: false,
        afterInvocation: false,
        beforeModelCall: false,
        afterModelCall: false,
        beforeToolCall: false,
        afterToolCall: false,
      },
      retry: n.retry ?? {
        kind: 'exponential' as const,
        maxAttempts: 3,
        baseDelayMs: 1000,
        maxDelayMs: 10000,
      },
      conversationManager: n.conversationManager ?? {
        kind: 'sliding-window' as const,
        windowSize: 20,
      },
      state: n.state ?? {
        enabled: false,
        type: 'stateless' as const,
      },
      storage: n.storage,
      limits: n.limits ?? {},
    })),
    edges: overrides.edges ?? [],
    metadata: overrides.metadata ?? {},
    createdAt: '2026-01-01T00:00:00.000Z',
    updatedAt: '2026-01-01T00:00:00.000Z',
  };

  return manifest;
}

export { ManifestSchema, SubCapabilityManifestSchema, ManifestEdgeSchema, GuardrailSpecSchema };
export { GUARDRAIL_TYPES, GUARDRAIL_FAILURE_MODES, RETRY_KINDS, MEMORY_TYPES, CONVERSATION_MANAGER_KINDS, STORAGE_KINDS };