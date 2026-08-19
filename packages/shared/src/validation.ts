import { z } from 'zod';

export const CreateWorkspaceSchema = z.object({
  name: z.string().min(1).max(255),
  organization: z.string().max(255).optional().default(''),
});
export const UpdateWorkspaceSchema = CreateWorkspaceSchema.partial();

export const CreateProjectSchema = z.object({
  workspaceId: z.string().uuid(),
  name: z.string().min(1).max(255),
  description: z.string().max(2000).optional().default(''),
});
export const UpdateProjectSchema = CreateProjectSchema.omit({ workspaceId: true }).partial();

export const CreateCapabilitySchema = z.object({
  projectId: z.string().uuid(),
  name: z.string().min(1).max(255),
  description: z.string().max(2000).optional().default(''),
});
export const UpdateCapabilitySchema = CreateCapabilitySchema.omit({ projectId: true }).partial();

export const PromptConfigSchema = z.object({
  systemPrompt: z.string().min(1),
  userTemplate: z.string().default(''),
});

export const ModelPolicySchema = z.object({
  provider: z.string().min(1),
  modelId: z.string().min(1),
  temperature: z.number().min(0).max(2).default(0.7),
  maxTokens: z.number().int().min(1).max(100000).default(4096),
  topP: z.number().min(0).max(1).optional(),
  stopSequences: z.array(z.string()).optional(),
});

export const RuntimePolicySchema = z.object({
  timeoutMs: z.number().int().min(1).max(600000).default(30000),
  nodeTimeoutMs: z.number().int().min(1).max(300000).default(10000),
  totalTimeoutMs: z.number().int().min(1).max(3600000).default(300000),
  maxRetries: z.number().int().min(0).max(10).default(3),
  canaryPercent: z.number().int().min(0).max(100).default(0),
  concurrencyLimit: z.number().int().min(1).max(1000).default(10),
});

export const ContextContractSchema = z.object({
  inputsSchema: z.record(z.string(), z.unknown()),
  outputsSchema: z.record(z.string(), z.unknown()),
  requiredContextVars: z.array(z.string()).default([]),
});

export const MemoryConfigSchema = z.object({
  enabled: z.boolean().default(false),
  type: z.enum(['stateless', 'session', 'persistent']).default('stateless'),
  retentionSec: z.number().int().min(0).optional(),
});

const GUARDRAIL_TYPES = ['regex', 'schema', 'llm-judge', 'blocklist', 'pii-redaction'] as const;
const GUARDRAIL_FAILURE_MODES = ['block', 'warn', 'redact'] as const;

export const GuardrailSpecSchema = z.object({
  type: z.enum(GUARDRAIL_TYPES),
  config: z.record(z.string(), z.unknown()),
  enabled: z.boolean().default(true),
  onFailure: z.enum(GUARDRAIL_FAILURE_MODES).default('block'),
});

export const ToolSpecSchema = z.object({
  name: z.string().min(1),
  description: z.string().min(1),
  inputSchema: z.record(z.string(), z.unknown()),
  config: z.record(z.string(), z.unknown()).default({}),
});

export const McpServerSpecSchema = z.object({
  name: z.string().min(1),
  url: z.string().url(),
  tools: z.array(z.string()).default([]),
  auth: z.record(z.string(), z.unknown()).optional(),
});

export const ObservabilityConfigSchema = z.object({
  logInputs: z.boolean().default(true),
  logOutputs: z.boolean().default(true),
  trackLatency: z.boolean().default(true),
  trackCost: z.boolean().default(true),
});

export const HookConfigSchema = z.object({
  beforeInvocation: z.boolean().default(false),
  afterInvocation: z.boolean().default(false),
  beforeModelCall: z.boolean().default(false),
  afterModelCall: z.boolean().default(false),
  beforeToolCall: z.boolean().default(false),
  afterToolCall: z.boolean().default(false),
});

export const RetrySpecSchema = z.object({
  kind: z.enum(['constant', 'linear', 'exponential']).default('exponential'),
  maxAttempts: z.number().int().min(1).max(10).default(3),
  baseDelayMs: z.number().int().min(100).default(1000),
  maxDelayMs: z.number().int().min(1000).default(10000),
});

export const LimitsSpecSchema = z.object({
  turns: z.number().int().min(1).optional(),
  outputTokens: z.number().int().min(1).optional(),
  totalTokens: z.number().int().min(1).optional(),
});

export const StateConfigSchema = z.object({
  enabled: z.boolean().default(false),
  type: z.enum(['stateless', 'session', 'persistent']).default('stateless'),
  retentionSec: z.number().int().min(0).optional(),
});

export const StorageConfigSchema = z.object({
  kind: z.enum(['test', 'custom']).default('test'),
  path: z.string().optional(),
});

export const ConversationManagerConfigSchema = z.object({
  kind: z.enum(['sliding-window', 'summarizing']).default('sliding-window'),
  windowSize: z.number().int().min(1).optional(),
  summaryRatio: z.number().min(0).max(1).optional(),
});

export const EvaluationConfigSchema = z.object({
  datasets: z.array(z.string()),
  scorers: z.array(z.string()),
  passThreshold: z.number().min(0).max(1),
  primaryScorer: z.string().optional(),
});

export const ManifestEdgeSchema = z.object({
  from: z.string().min(1),
  to: z.string().min(1),
  mapping: z.record(z.string(), z.string()).default({}),
});

export type ManifestNodeInput = z.infer<typeof SubCapabilityManifestSchema>;

export const SubCapabilityManifestSchema: z.ZodType<unknown> = z.lazy(() =>
  z.object({
    id: z.string().min(1),
    name: z.string().min(1),
    description: z.string().default(''),
    goal: z.string().min(1),
    manifest: ManifestSchema,
    dependsOn: z.array(z.string()).default([]),
    preGuardrails: z.array(GuardrailSpecSchema).default([]),
    postGuardrails: z.array(GuardrailSpecSchema).default([]),
    observability: ObservabilityConfigSchema,
    hooks: HookConfigSchema,
    retry: RetrySpecSchema,
    conversationManager: ConversationManagerConfigSchema,
    state: StateConfigSchema,
    storage: StorageConfigSchema.optional(),
    limits: LimitsSpecSchema,
  }),
);

export const ManifestSchema = z.object({
  id: z.string().min(1),
  version: z.number().int().min(1),
  prompt: PromptConfigSchema,
  model: ModelPolicySchema,
  runtime: RuntimePolicySchema,
  context: ContextContractSchema,
  memory: MemoryConfigSchema,
  guardrails: z.object({
    pre: z.array(GuardrailSpecSchema).default([]),
    post: z.array(GuardrailSpecSchema).default([]),
  }),
  tools: z.array(ToolSpecSchema).default([]),
  mcpServers: z.array(McpServerSpecSchema).default([]),
  evaluation: EvaluationConfigSchema,
  nodes: z.array(SubCapabilityManifestSchema).default([]),
  edges: z.array(ManifestEdgeSchema).default([]),
  metadata: z.record(z.string(), z.unknown()).default({}),
  createdAt: z.string().default(''),
  updatedAt: z.string().default(''),
}).superRefine((manifest, ctx) => {
  validateDag(manifest as unknown as { nodes: Array<{ id: string }>; edges: Array<{ from: string; to: string }> }, ctx);
});

function validateDag(
  manifest: { nodes: Array<{ id: string }>; edges: Array<{ from: string; to: string }> },
  ctx: z.RefinementCtx,
): void {
  const ids = new Set(manifest.nodes.map((n) => n.id));
  for (const node of manifest.nodes) {
    if (!node.id) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: 'Node id must not be empty',
      });
    }
  }
  for (const edge of manifest.edges) {
    if (!ids.has(edge.from)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: `Edge references missing source node: ${edge.from}`,
      });
    }
    if (!ids.has(edge.to)) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: `Edge references missing target node: ${edge.to}`,
      });
    }
  }
  const adj = new Map<string, string[]>();
  for (const node of manifest.nodes) adj.set(node.id, []);
  for (const edge of manifest.edges) {
    adj.get(edge.from)?.push(edge.to);
  }
  const color = new Map<string, 0 | 1 | 2>();
  for (const node of manifest.nodes) color.set(node.id, 0);
  function visit(node: string, stack: string[]): void {
    const c = color.get(node);
    if (c === 1) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        message: `DAG cycle detected: ${[...stack, node].join(' -> ')}`,
      });
      return;
    }
    if (c === 2) return;
    color.set(node, 1);
    for (const next of adj.get(node) ?? []) {
      visit(next, [...stack, node]);
    }
    color.set(node, 2);
  }
  for (const node of manifest.nodes) {
    if (color.get(node.id) === 0) visit(node.id, []);
  }
}

export const CreateReleaseSchema = z.object({
  capabilityId: z.string().uuid(),
  capabilityVersion: z.number().int().positive(),
  environment: z.enum(['dev', 'staging', 'prod']),
  canaryPercent: z.number().int().min(0).max(100).optional().default(0),
});
export const ActivateReleaseSchema = z.object({
  releaseId: z.string().uuid(),
});
export const SupersedeReleaseSchema = z.object({
  releaseId: z.string().uuid(),
  supersededBy: z.string().uuid(),
});

export const VoteApprovalSchema = z.object({
  releaseId: z.string().uuid(),
  voter: z.string().min(1),
  approved: z.boolean(),
  comment: z.string().max(1000).optional().default(''),
});

export const InvokeExecutionSchema = z.object({
  capabilityVersionId: z.string().uuid(),
  inputs: z.record(z.string(), z.unknown()),
  environment: z.string().optional().default(''),
  traceId: z.string().optional().default(''),
});

export const CreateDatasetSchema = z.object({
  capabilityId: z.string().uuid(),
  name: z.string().min(1).max(255),
  description: z.string().max(2000).optional().default(''),
});
export const CreateDatasetCaseSchema = z.object({
  inputs: z.record(z.string(), z.unknown()),
  expected: z.record(z.string(), z.unknown()),
  description: z.string().max(2000).optional().default(''),
});

export const CreateEvalRunSchema = z.object({
  releaseId: z.string().uuid(),
  datasetId: z.string().uuid(),
  scorer: z.string().min(1),
});

export const CreateAlertRuleSchema = z.object({
  name: z.string().min(1).max(255),
  type: z.string().min(1),
  severity: z.enum(['info', 'warning', 'critical']),
  enabled: z.boolean().default(true),
  threshold: z.number().default(0),
  duration: z.number().int().default(0),
  window: z.number().int().default(0),
  config: z.record(z.string(), z.unknown()).optional(),
});
export const UpdateAlertRuleSchema = CreateAlertRuleSchema.partial();

export const CreatePreconditionSchema = z.object({
  capabilityId: z.string().uuid(),
  name: z.string().min(1).max(255),
  command: z.string().min(1),
  timeoutSec: z.number().int().min(1).max(3600).default(60),
  enabled: z.boolean().default(true),
});

export const CreateScheduleSchema = z.object({
  workspaceId: z.string().uuid(),
  releaseId: z.string().uuid(),
  kind: z.string().min(1),
  cron: z.string().min(1),
  enabled: z.boolean().default(true),
});

export const CreateUserSchema = z.object({
  email: z.string().email(),
  name: z.string().min(1).max(255),
  role: z.enum(['admin', 'editor', 'reader']).default('reader'),
});
export const UpdateUserSchema = CreateUserSchema.partial();

export const CreateApiKeySchema = z.object({
  name: z.string().min(1).max(255),
  role: z.enum(['admin', 'editor', 'reader']).default('reader'),
  expiresAt: z.string().datetime().optional(),
});

export const CreateProviderKeySchema = z.object({
  providerName: z.string().min(1),
  keyName: z.string().min(1),
  encryptedKey: z.string().min(1),
});

export const CreateWebhookSchema = z.object({
  url: z.string().url(),
  events: z.string().min(1),
  active: z.boolean().default(true),
});

export const CreateNotificationGroupSchema = z.object({
  name: z.string().min(1).max(255),
  channels: z.array(z.string()).default([]),
});

export const CreateRecommendationSchema = z.object({
  capabilityVersionId: z.string().uuid(),
  type: z.string().min(1),
  payload: z.record(z.string(), z.unknown()),
});
export const CreateDecisionSchema = z.object({
  recommendationId: z.string().uuid(),
  payload: z.record(z.string(), z.unknown()),
});

export const CreateLineageEdgeSchema = z.object({
  capabilityId: z.string().uuid(),
  parentCapabilityId: z.string().uuid(),
  parentVersion: z.number().int().positive(),
  childCapabilityId: z.string().uuid(),
  childVersion: z.number().int().positive(),
  source: z.enum(['recommendation', 'manual', 'migration']),
  recommendationId: z.string().uuid().optional(),
  notes: z.record(z.string(), z.unknown()).optional().default({}),
});

export const PaginationSchema = z.object({
  page: z.number().int().min(1).default(1),
  pageSize: z.number().int().min(1).max(100).default(20),
});
