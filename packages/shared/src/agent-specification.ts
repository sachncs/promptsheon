import { createHash } from 'node:crypto';
import { z } from 'zod';

/** The schema version encoded into every agent specification. */
export const AGENT_SPECIFICATION_VERSION = '1.0' as const;

const JsonObjectSchema = z.record(z.string(), z.unknown());

const NonEmptyString = (max: number) => z.string().trim().min(1).max(max);

const GuardrailSchema = z.strictObject({
  id: NonEmptyString(128),
  kind: z.enum(['input', 'output', 'tool', 'policy', 'content']),
  rule: NonEmptyString(2000),
  failureMode: z.enum(['block', 'warn', 'redact']).default('block'),
  config: JsonObjectSchema.default({}),
});

const ToolSchema = z.strictObject({
  name: NonEmptyString(128),
  description: NonEmptyString(2000),
  inputSchema: JsonObjectSchema,
});

const PermissionSchema = z.strictObject({
  allowedTools: z.array(NonEmptyString(128)).max(256).default([]),
  network: z.enum(['none', 'restricted', 'unrestricted']).default('none'),
  filesystem: z.enum(['none', 'workspace', 'restricted']).default('none'),
  secrets: z.enum(['none', 'declared']).default('none'),
});

const ModelPolicySchema = z.strictObject({
  provider: NonEmptyString(128),
  model: NonEmptyString(256),
  temperature: z.number().finite().min(0).max(2).default(0),
  maxOutputTokens: z.number().int().min(1).max(1_000_000).default(4096),
  topP: z.number().finite().min(0).max(1).default(1),
});

const RoutingPolicySchema = z.strictObject({
  strategy: z.enum(['fixed', 'fallback', 'latency', 'cost', 'quality']).default('fixed'),
  fallbackModels: z.array(NonEmptyString(256)).max(32).default([]),
});

const ExecutionPolicySchema = z.strictObject({
  timeoutMs: z.number().int().min(1).max(3_600_000).default(120_000),
  maxAttempts: z.number().int().min(1).max(10).default(1),
  concurrency: z.number().int().min(1).max(1_000).default(1),
  deterministic: z.boolean().default(true),
});

const MemoryPolicySchema = z.strictObject({
  mode: z.enum(['none', 'session', 'persistent']).default('none'),
  retentionSeconds: z.number().int().min(0).max(31_536_000).default(0),
});

const EvaluationPolicySchema = z.strictObject({
  suites: z.array(NonEmptyString(256)).max(256).default([]),
  requiredScore: z.number().finite().min(0).max(1).default(0),
});

const ResourceBudgetSchema = z.strictObject({
  maxInputTokens: z.number().int().min(1).max(10_000_000).default(100_000),
  maxOutputTokens: z.number().int().min(1).max(1_000_000).default(16_000),
  maxCostUsd: z.number().finite().min(0).max(1_000_000).default(0),
  maxWallTimeMs: z.number().int().min(1).max(86_400_000).default(3_600_000),
});

const LifecycleSchema = z.strictObject({
  stage: z.enum(['draft', 'candidate', 'published', 'retired']).default('draft'),
  owner: NonEmptyString(256),
});

/**
 * The immutable, content-addressed definition of an agent.
 *
 * Runtime timestamps, revision identifiers, and storage metadata are
 * intentionally absent. They belong to the published revision record.
 */
export const AgentSpecificationSchema = z.strictObject({
  schemaVersion: z.literal(AGENT_SPECIFICATION_VERSION).default(AGENT_SPECIFICATION_VERSION),
  role: NonEmptyString(500),
  objective: NonEmptyString(4_000),
  prompt: z.strictObject({
    system: NonEmptyString(100_000),
    developer: z.string().trim().max(100_000).default(''),
    template: z.string().trim().max(100_000).default(''),
  }),
  contextPolicy: z.strictObject({
    required: z.array(NonEmptyString(256)).max(256).default([]),
    maxInputTokens: z.number().int().min(1).max(10_000_000).default(100_000),
  }).default({ required: [], maxInputTokens: 100_000 }),
  guardrails: z.array(GuardrailSchema).max(512).default([]),
  capabilities: z.strictObject({
    tools: z.array(ToolSchema).max(256).default([]),
    delegation: z.boolean().default(false),
  }).default({ tools: [], delegation: false }),
  permissions: PermissionSchema.default({ allowedTools: [], network: 'none', filesystem: 'none', secrets: 'none' }),
  modelPolicy: ModelPolicySchema,
  routingPolicy: RoutingPolicySchema.default({ strategy: 'fixed', fallbackModels: [] }),
  executionPolicy: ExecutionPolicySchema.default({ timeoutMs: 120_000, maxAttempts: 1, concurrency: 1, deterministic: true }),
  memoryPolicy: MemoryPolicySchema.default({ mode: 'none', retentionSeconds: 0 }),
  evaluationPolicy: EvaluationPolicySchema.default({ suites: [], requiredScore: 0 }),
  resourceBudget: ResourceBudgetSchema.default({ maxInputTokens: 100_000, maxOutputTokens: 16_000, maxCostUsd: 0, maxWallTimeMs: 3_600_000 }),
  lifecycle: LifecycleSchema,
  metadata: JsonObjectSchema.default({}),
});

export type AgentSpecification = z.infer<typeof AgentSpecificationSchema>;

const MAX_CANONICAL_DEPTH = 64;

/** Canonical JSON is the hash input: sorted keys, NFC strings, and no whitespace. */
export function canonicalizeSpecification(specification: AgentSpecification): string {
  const parsed = AgentSpecificationSchema.parse(specification);
  return JSON.stringify(sortCanonicalValue(parsed, 0));
}

/** Compute the documented SHA-256 content address of an agent specification. */
export function hashAgentSpecification(specification: AgentSpecification): string {
  return createHash('sha256').update(canonicalizeSpecification(specification), 'utf8').digest('hex');
}

function sortCanonicalValue(value: unknown, depth: number): unknown {
  if (depth > MAX_CANONICAL_DEPTH) throw new Error('agent specification exceeds maximum nesting depth');
  if (typeof value === 'string') return value.normalize('NFC').replace(/\r\n?/g, '\n');
  if (Array.isArray(value)) return value.map((item) => sortCanonicalValue(item, depth + 1));
  if (value && typeof value === 'object') {
    const record = value as Record<string, unknown>;
    return Object.fromEntries(
      Object.keys(record)
        .sort()
        .map((key) => [key, sortCanonicalValue(record[key], depth + 1)]),
    );
  }
  return value;
}
