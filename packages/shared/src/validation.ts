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

export const ManifestSchema = z.object({
  systemPrompt: z.string().min(1),
  model: z.string().min(1),
  provider: z.string().min(1),
  temperature: z.number().min(0).max(2).default(0.7),
  maxTokens: z.number().int().min(1).max(100000).default(4096),
  tools: z.array(z.object({
    name: z.string(),
    enabled: z.boolean().default(true),
    config: z.record(z.unknown()).default({}),
  })).default([]),
  metadata: z.record(z.unknown()).default({}),
});

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
  inputs: z.record(z.unknown()),
  environment: z.string().optional().default(''),
  traceId: z.string().optional().default(''),
});

export const CreateDatasetSchema = z.object({
  capabilityId: z.string().uuid(),
  name: z.string().min(1).max(255),
  description: z.string().max(2000).optional().default(''),
});
export const CreateDatasetCaseSchema = z.object({
  inputs: z.record(z.unknown()),
  expected: z.record(z.unknown()),
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
  config: z.record(z.unknown()).optional(),
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
  payload: z.record(z.unknown()),
});
export const CreateDecisionSchema = z.object({
  recommendationId: z.string().uuid(),
  payload: z.record(z.unknown()),
});

export const CreateLineageEdgeSchema = z.object({
  capabilityId: z.string().uuid(),
  parentCapabilityId: z.string().uuid(),
  parentVersion: z.number().int().positive(),
  childCapabilityId: z.string().uuid(),
  childVersion: z.number().int().positive(),
  source: z.enum(['recommendation', 'manual', 'migration']),
  recommendationId: z.string().uuid().optional(),
  notes: z.record(z.unknown()).optional().default({}),
});

export const PaginationSchema = z.object({
  page: z.number().int().min(1).default(1),
  pageSize: z.number().int().min(1).max(100).default(20),
});
