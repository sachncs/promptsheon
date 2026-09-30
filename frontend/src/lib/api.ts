import axios from 'axios';
import { z } from 'zod';
import { ManifestSchema } from '@promptsheon/shared/validation';
import type { Manifest } from '@promptsheon/shared';
import type { Execution } from '@promptsheon/shared';
import type { Schedule } from '@promptsheon/shared';
import type { Dataset, DatasetCase } from '@promptsheon/shared';
import type { AlertRule, Precondition } from '@promptsheon/shared';
import type { User, UserRole } from '@promptsheon/shared';
import type { FeatureFlag } from '@promptsheon/shared';

export type { Dataset, DatasetCase };
export type { AlertRule, Precondition };
export type { User, UserRole };
export type { FeatureFlag };
import { clearSession } from './session';

export class ApiError extends Error {
  readonly status: number | undefined;
  readonly code: string | undefined;

  constructor(message: string, options: { status?: number | undefined; code?: string | undefined } = {}) {
    super(message);
    this.name = 'ApiError';
    this.status = options.status;
    this.code = options.code;
  }
}

export { getErrorMessage } from './errors';

const client = axios.create({
  baseURL: '/api',
  timeout: 15_000,
  headers: { 'Content-Type': 'application/json' },
});

client.interceptors.request.use((config) => {
  return config;
});

client.interceptors.response.use(
  (res) => res,
  (err) => {
    const status = typeof err.response?.status === 'number' ? err.response.status : undefined;
    const payload = err.response?.data?.error;
    const message = typeof payload?.message === 'string'
      ? payload.message
      : err.code === 'ECONNABORTED'
        ? 'The request timed out. Please try again.'
        : err.message || 'The request failed.';
    if (status === 401) clearSession();
    return Promise.reject(new ApiError(message, {
      status,
      code: typeof payload?.code === 'string' ? payload.code : undefined,
    }));
  },
);

export { client };

export const authApi = {
  establish: async (apiKey: string): Promise<void> => {
    await client.post('/auth/session', {}, { headers: { Authorization: `Bearer ${apiKey}` } });
  },
  logout: async (): Promise<void> => {
    await client.delete('/auth/logout');
  },
};

export interface WorkspaceRow {
  id: string;
  name: string;
  organization: string;
  createdAt: string;
  updatedAt: string;
}

export type AgentSpecificationStatus = 'draft' | 'candidate' | 'published' | 'retired';

export interface AgentSpecificationMetadata {
  hash: string;
  workspaceId: string;
  schemaVersion: string;
  parentHash: string | null;
  author: string;
  changeReason: string;
  status: AgentSpecificationStatus;
  createdAt: string;
  publishedAt: string | null;
}

export interface AgentSpecificationList {
  items: AgentSpecificationMetadata[];
  total: number;
}

export interface AgentSpecificationRecord extends AgentSpecificationMetadata {
  specification: unknown;
  security?: AgentSpecificationSecurityReport | undefined;
}

export interface AgentSpecificationSecurityFinding {
  rule: string;
  severity: 'info' | 'warn' | 'block';
  message: string;
  range?: { start: number; end: number } | undefined;
  snippet?: string | undefined;
}

export interface AgentSpecificationSecurityReport {
  verdict: 'clean' | 'warn' | 'block';
  findings: AgentSpecificationSecurityFinding[];
}

export interface AgentSpecificationDraft {
  role: string;
  objective: string;
  prompt: { system: string };
  modelPolicy: { provider: string; model: string };
  lifecycle: { owner: string };
}

export interface AgentSpecificationValidationIssue {
  code: string;
  message: string;
  path: Array<string | number>;
}

export interface AgentSpecificationDiffEntry {
  path: string;
  before: unknown;
  after: unknown;
}

export interface VaultKeyringEntry {
  id: number;
  label: string;
  fingerprint: string;
  active: boolean;
  createdAt: string;
  rotatedAt: string | null;
}

export interface VaultSecretMetadata {
  id: string;
  organizationId: string;
  name: string;
  fingerprint: string;
  createdBy: string;
  createdAt: string;
  rotatedAt: string | null;
}

export interface VaultKeyRotation {
  key: VaultKeyringEntry;
  reencrypted: number;
}

export interface SigningKey {
  id: string;
  organizationId: string;
  label: string;
  fingerprint: string;
  publicKeyPem: string;
  createdBy: string;
  createdAt: string;
  deactivatedAt: string | null;
}

const SigningKeySchema = z.object({
  id: z.string().min(1),
  organizationId: z.string().min(1),
  label: z.string().min(1),
  fingerprint: z.string().min(1),
  publicKeyPem: z.string().min(1),
  createdBy: z.string().min(1),
  createdAt: z.string(),
  deactivatedAt: z.string().nullable(),
});

const SigningKeyListSchema = z.array(SigningKeySchema);

const VaultKeyringEntrySchema = z.object({
  id: z.number().int(),
  label: z.string(),
  fingerprint: z.string(),
  active: z.boolean(),
  createdAt: z.string(),
  rotatedAt: z.string().nullable(),
});

const VaultSecretMetadataSchema = z.object({
  id: z.string().min(1),
  organizationId: z.string().min(1),
  name: z.string().min(1),
  fingerprint: z.string().min(1),
  createdBy: z.string().min(1),
  createdAt: z.string(),
  rotatedAt: z.string().nullable(),
});

const VaultKeyRotationSchema = z.object({
  key: VaultKeyringEntrySchema,
  reencrypted: z.number().int().nonnegative(),
});

export interface CostRollup {
  capabilityId: string;
  day: string;
  costMicros: number;
  executions: number;
}

export interface EvalSuite {
  id: string;
  capabilityId: string;
  repositoryId: string | null;
  name: string;
  description: string | null;
  currentVersion: number;
  passThreshold: number;
  borderlineBand: number;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
}

export interface EvalSuiteVersion {
  id: string;
  suiteId: string;
  version: number;
  graderConfig: unknown[];
  passThreshold: number;
  borderlineBand: number;
  k: number;
  n: number;
  notes: string | null;
  createdBy: string;
  createdAt: string;
}

export interface EvalSuiteRun {
  id: string;
  suiteId: string;
  suiteVersionId: string;
  n: number;
  k: number;
  passAtK: number;
  rawScore: number;
  passed: boolean;
  borderlineCount: number;
  status: 'running' | 'completed' | 'failed';
  startedAt: string;
  finishedAt: string | null;
  error: string | null;
}

export interface MutationProposal {
  id: string;
  organizationId: string;
  workspaceId: string | null;
  sourceHash: string;
  candidateHash: string | null;
  mutationKind: 'prompt' | 'guardrail' | 'model' | 'routing' | 'context' | 'tool' | 'permission' | 'execution' | 'memory' | 'budget';
  changes: Record<string, unknown>;
  rationale: string;
  expectedOutcome: string;
  authorType: 'human' | 'system' | 'simulator';
  authorId: string;
  risk: 'low' | 'medium' | 'high' | 'critical';
  confidence: number;
  baselineScore: number | null;
  candidateScore: number | null;
  evaluationStatus: 'pending' | 'passed' | 'failed';
  status: 'proposed' | 'validated' | 'approved' | 'rejected' | 'abandoned';
  evaluationRunId: string | null;
  decisionReason: string | null;
  reviewedBy: string | null;
  reviewedAt: string | null;
  createdAt: string;
  updatedAt: string;
  promotedReleaseId: string | null;
  promotedAt: string | null;
}

export type EvalRunStatus = 'running' | 'passed' | 'failed' | 'error';

export interface EvalRun {
  id: string;
  releaseId: string;
  datasetId: string;
  scorer: string;
  score: number;
  passed: number;
  failed: number;
  total: number;
  status: EvalRunStatus;
  startedAt: string;
  finishedAt: string | null;
}

export interface EvalResult {
  id: string;
  runId: string;
  caseId: string | null;
  seq: number;
  passed: boolean;
  actual: string;
  error: string;
  latencyMs: number;
}

export type MergeRequestStatus = 'open' | 'merged' | 'closed';

export interface MergeRequest {
  id: string;
  repositoryId: string;
  number: number;
  title: string;
  description: string | null;
  sourceBranch: string;
  targetBranch: string;
  sourceCommitOid: string;
  mergeCommitOid: string | null;
  authorId: string;
  status: MergeRequestStatus;
  approvedBy: string[];
  requestedReviewers: string[];
  createdAt: string;
  updatedAt: string;
  mergedAt: string | null;
}

export interface MergeRequestApproval {
  mergeRequestId: string;
  userId: string;
  decision: 'approve' | 'request_changes';
  commentId: string | null;
  createdAt: string;
}

export interface MergeRequestComment {
  id: string;
  mergeRequestId: string;
  authorId: string;
  path: string | null;
  body: string;
  createdAt: string;
}

export interface CapabilityVersion {
  id: string;
  capabilityId: string;
  version: number;
  manifest: string;
  manifestHash: string;
  createdAt: string;
  createdBy: string;
}

export interface CapabilityManifestResponse {
  id: string;
  hash: string;
  manifest: unknown;
  capabilityId: string;
  capabilityVersion: number;
  createdAt: string;
  createdBy: string;
  size: number;
}

export interface SearchResult {
  kind: string;
  resourceId: string;
  title: string;
  body: string;
}

export type AlertSeverity = 'info' | 'warning' | 'critical';
export type AlertStatus = 'active' | 'resolved';

export interface Alert {
  id: string;
  ruleId: string | null;
  ruleName: string;
  severity: AlertSeverity;
  status: AlertStatus;
  message: string;
  details: string | null;
  triggeredAt: string;
  resolvedAt: string | null;
  acknowledgedAt: string | null;
  acknowledgedBy: string | null;
}

export interface SelfEvolveState {
  status: 'idle' | 'detected' | 'revising' | 'validating' | 'promoted' | 'rejected';
  lastRevisionHash?: string;
  lastEvalScore?: number;
  cycleCount: number;
}

export interface SelfEvolveCycle {
  action: 'revised' | 'no_change';
  state: SelfEvolveState;
}

export interface GoalSummary {
  manifestHash: string;
  bestScore: number;
  iterations: number;
  lastUpdated: string;
}

export interface GoalHistoryEntry {
  iteration: number;
  score: number;
  cost: number;
  revised: boolean;
  timestamp: string;
}

export interface GoalSnapshot {
  iteration: number;
  manifestHash: string;
  score: number;
  timestamp: string;
}

export interface GoalDetail {
  manifestHash: string;
  bestScore: number;
  bestManifestHash: string;
  iterations: number;
  totalCost: number;
  snapshots: GoalSnapshot[];
  history: GoalHistoryEntry[];
}

export interface SecurityFinding {
  rule: string;
  severity: 'info' | 'warn' | 'block';
  message: string;
  snippet: string | null;
  range: { start: number; end: number } | null;
}

export interface SecurityScanResult {
  verdict: 'clean' | 'warn' | 'block';
  findings: SecurityFinding[];
}

export interface SecurityScanSummary {
  orgId: string;
  days: number;
  total: number;
  byVerdict: { clean: number; warn: number; block: number };
}

export interface SettingItem {
  key: string;
  value: unknown;
}

export type ApiKeyRole = 'admin' | 'editor' | 'reader' | 'system';

export interface ApiKeySummary {
  id: string;
  userId: string;
  name: string;
  keyPrefix: string;
  role: ApiKeyRole;
  expiresAt: string | null;
  lastUsed: string | null;
  createdAt: string;
  revoked: boolean;
}

export interface IssuedApiKey {
  key: string;
  id: string;
  name: string;
}

export interface WebhookSubscription {
  id: string;
  organizationId: string;
  label: string;
  url: string;
  events: string[];
  active: boolean;
  createdAt: string;
  updatedAt: string;
  createdBy: string;
}

export interface Capability {
  id: string;
  projectId: string;
  name: string;
  description: string;
  createdAt: string;
  updatedAt: string;
  selfEvolveEnabled: boolean;
  selfEvolveMinScore: number;
  selfEvolveMaxRevisions: number;
  selfEvolveCooldownSec: number;
  selfEvolveTargetEnv: string;
  selfEvolveDatasetId: string;
}

export interface Project {
  id: string;
  workspaceId: string;
  name: string;
  description: string;
  createdAt: string;
  updatedAt: string;
}

export type ReleaseStatus = 'draft' | 'review' | 'approved' | 'canary' | 'active' | 'rolled_back';
export type ReleaseEnvironment = 'dev' | 'staging' | 'prod';

export interface Release {
  id: string;
  capabilityId: string;
  capabilityVersion: number;
  capabilityVersionId: string | null;
  manifest: string;
  environment: ReleaseEnvironment;
  status: ReleaseStatus;
  approvedBy: string;
  replacesReleaseId: string | null;
  createdAt: string;
  createdBy: string;
  activatedAt: string | null;
  canaryPercent: number;
  signature?: string | null | undefined;
  signedKeyId?: string | null | undefined;
  signedAt?: string | null | undefined;
}

export type ApprovalVote = 'approve' | 'reject';

export interface ApprovalEntry {
  userId: string;
  vote: ApprovalVote;
  comment: string;
  createdAt: string;
}

export interface ApprovalSummary {
  releaseId: string;
  manifestHash?: string | undefined;
  distinctApprovers: number;
  approvals: ApprovalEntry[];
}

export interface PendingApprovalSummary {
  releaseId: string;
  manifestHash: string;
  approvals: ApprovalEntry[];
  updatedAt: string;
}

export interface AuditEntry {
  id: string;
  userId: string;
  action: string;
  resource: string;
  details: string;
  timestamp: string;
  entryHash: string;
  resourceKind: string;
  resourceId: string;
}

export type { Execution };
export type { Schedule };

const CostRollupSchema = z.object({
  capabilityId: z.string(),
  day: z.string(),
  costMicros: z.number().int().nonnegative(),
  executions: z.number().int().nonnegative(),
});

const EvalSuiteSchema = z.object({
  id: z.string(),
  capabilityId: z.string(),
  repositoryId: z.string().nullable(),
  name: z.string(),
  description: z.string().nullable(),
  currentVersion: z.number().int().positive(),
  passThreshold: z.number().min(0).max(1),
  borderlineBand: z.number().min(0).max(1),
  createdBy: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

const EvalSuiteVersionSchema = z.object({
  id: z.string(),
  suiteId: z.string(),
  version: z.number().int().positive(),
  graderConfig: z.array(z.unknown()),
  passThreshold: z.number().min(0).max(1),
  borderlineBand: z.number().min(0).max(1),
  k: z.number().int().nonnegative(),
  n: z.number().int().nonnegative(),
  notes: z.string().nullable(),
  createdBy: z.string(),
  createdAt: z.string(),
});

const EvalSuiteRunSchema = z.object({
  id: z.string(),
  suiteId: z.string(),
  suiteVersionId: z.string(),
  n: z.number().int().positive(),
  k: z.number().int().positive(),
  passAtK: z.number().min(0).max(1),
  rawScore: z.number().min(0).max(1),
  passed: z.boolean(),
  borderlineCount: z.number().int().nonnegative(),
  status: z.enum(['running', 'completed', 'failed']),
  startedAt: z.string(),
  finishedAt: z.string().nullable(),
  error: z.string().nullable(),
});

const MutationProposalSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  workspaceId: z.string().nullable(),
  sourceHash: z.string(),
  candidateHash: z.string().nullable(),
  mutationKind: z.enum(['prompt', 'guardrail', 'model', 'routing', 'context', 'tool', 'permission', 'execution', 'memory', 'budget']),
  changes: z.record(z.string(), z.unknown()),
  rationale: z.string(),
  expectedOutcome: z.string(),
  authorType: z.enum(['human', 'system', 'simulator']),
  authorId: z.string(),
  risk: z.enum(['low', 'medium', 'high', 'critical']),
  confidence: z.number().min(0).max(1),
  baselineScore: z.number().min(0).max(1).nullable(),
  candidateScore: z.number().min(0).max(1).nullable(),
  evaluationStatus: z.enum(['pending', 'passed', 'failed']),
  status: z.enum(['proposed', 'validated', 'approved', 'rejected', 'abandoned']),
  evaluationRunId: z.string().nullable(),
  decisionReason: z.string().nullable(),
  reviewedBy: z.string().nullable(),
  reviewedAt: z.string().nullable(),
  createdAt: z.string(),
  updatedAt: z.string(),
  promotedReleaseId: z.string().nullable(),
  promotedAt: z.string().nullable(),
});

const EvalRunSchema = z.object({
  id: z.string(),
  releaseId: z.string(),
  datasetId: z.string(),
  scorer: z.string(),
  score: z.number().min(0).max(1),
  passed: z.number().int().nonnegative(),
  failed: z.number().int().nonnegative(),
  total: z.number().int().nonnegative(),
  status: z.enum(['running', 'passed', 'failed', 'error']),
  startedAt: z.string(),
  finishedAt: z.string().nullable(),
});

const EvalResultSchema = z.object({
  id: z.string(),
  runId: z.string(),
  caseId: z.string().nullable(),
  seq: z.number().int().nonnegative(),
  passed: z.boolean(),
  actual: z.string(),
  error: z.string(),
  latencyMs: z.number().nonnegative(),
});

const MergeRequestSchema = z.object({
  id: z.string(),
  repositoryId: z.string(),
  number: z.number().int().positive(),
  title: z.string(),
  description: z.string().nullable(),
  sourceBranch: z.string(),
  targetBranch: z.string(),
  sourceCommitOid: z.string(),
  mergeCommitOid: z.string().nullable(),
  authorId: z.string(),
  status: z.enum(['open', 'merged', 'closed']),
  approvedBy: z.array(z.string()),
  requestedReviewers: z.array(z.string()),
  createdAt: z.string(),
  updatedAt: z.string(),
  mergedAt: z.string().nullable(),
});

const MergeRequestApprovalSchema = z.object({
  mergeRequestId: z.string(),
  userId: z.string(),
  decision: z.enum(['approve', 'request_changes']),
  commentId: z.string().nullable(),
  createdAt: z.string(),
});

const MergeRequestCommentSchema = z.object({
  id: z.string(),
  mergeRequestId: z.string(),
  authorId: z.string(),
  path: z.string().nullable(),
  body: z.string(),
  createdAt: z.string(),
});

const CapabilityVersionSchema = z.object({
  id: z.string(),
  capabilityId: z.string(),
  version: z.number().int().positive(),
  manifest: z.string(),
  manifestHash: z.string(),
  createdAt: z.string(),
  createdBy: z.string(),
});

const CapabilityManifestResponseSchema = z.object({
  id: z.string(),
  hash: z.string(),
  manifest: z.unknown(),
  capabilityId: z.string(),
  capabilityVersion: z.number().int().positive(),
  createdAt: z.string(),
  createdBy: z.string(),
  size: z.number().int().nonnegative(),
});

const SearchResultSchema = z.object({
  kind: z.string(),
  resourceId: z.string(),
  title: z.string(),
  body: z.string(),
});

const AlertSchema = z.object({
  id: z.string(),
  ruleId: z.string().nullable(),
  ruleName: z.string(),
  severity: z.enum(['info', 'warning', 'critical']),
  status: z.enum(['active', 'resolved']),
  message: z.string(),
  details: z.string().nullable(),
  triggeredAt: z.string(),
  resolvedAt: z.string().nullable(),
  acknowledgedAt: z.string().nullable(),
  acknowledgedBy: z.string().nullable(),
});

const SelfEvolveStateSchema = z.object({
  status: z.enum(['idle', 'detected', 'revising', 'validating', 'promoted', 'rejected']),
  lastRevisionHash: z.string().optional(),
  lastEvalScore: z.number().optional(),
  cycleCount: z.number().int().nonnegative(),
});

const SelfEvolveCycleSchema = z.object({
  action: z.enum(['revised', 'no_change']),
  state: SelfEvolveStateSchema,
});

const GoalSummarySchema = z.object({
  manifestHash: z.string(),
  bestScore: z.number(),
  iterations: z.number().int().nonnegative(),
  lastUpdated: z.string(),
});

const GoalHistoryEntrySchema = z.object({
  iteration: z.number().int().nonnegative(),
  score: z.number(),
  cost: z.number(),
  revised: z.boolean(),
  timestamp: z.string(),
});

const GoalSnapshotSchema = z.object({
  iteration: z.number().int().nonnegative(),
  manifestHash: z.string(),
  score: z.number(),
  timestamp: z.string(),
});

const GoalDetailSchema = z.object({
  manifestHash: z.string(),
  bestScore: z.number(),
  bestManifestHash: z.string(),
  iterations: z.number().int().nonnegative(),
  totalCost: z.number(),
  snapshots: z.array(GoalSnapshotSchema),
  history: z.array(GoalHistoryEntrySchema),
});

const SecurityFindingSchema = z.object({
  rule: z.string(),
  severity: z.enum(['info', 'warn', 'block']),
  message: z.string(),
  snippet: z.string().optional(),
  range: z.object({ start: z.number().int().nonnegative(), end: z.number().int().nonnegative() }).optional(),
});

const SecurityScanResultSchema = z.object({
  verdict: z.enum(['clean', 'warn', 'block']),
  findings: z.array(SecurityFindingSchema),
});

const SecurityScanSummarySchema = z.object({
  orgId: z.string(),
  days: z.number().int().positive(),
  total: z.number().int().nonnegative(),
  byVerdict: z.object({
    clean: z.number().int().nonnegative(),
    warn: z.number().int().nonnegative(),
    block: z.number().int().nonnegative(),
  }),
});

const SettingItemSchema = z.object({
  key: z.string().min(1),
  value: z.unknown(),
});

const ApiKeySummarySchema = z.object({
  id: z.string().min(1),
  userId: z.string().min(1),
  name: z.string(),
  keyPrefix: z.string().min(1),
  role: z.enum(['admin', 'editor', 'reader', 'system']),
  expiresAt: z.string().nullable(),
  lastUsed: z.string().nullable(),
  createdAt: z.string(),
  revoked: z.boolean(),
});

const IssuedApiKeySchema = z.object({
  key: z.string().min(1),
  id: z.string().min(1),
  name: z.string(),
});

const WebhookSubscriptionSchema = z.object({
  id: z.string().min(1),
  organizationId: z.string().min(1),
  label: z.string().min(1),
  url: z.string().url(),
  events: z.array(z.string().min(1)),
  active: z.boolean(),
  createdAt: z.string(),
  updatedAt: z.string(),
  createdBy: z.string().min(1),
});

const UserSchema = z.object({
  id: z.string().min(1),
  email: z.string().email(),
  name: z.string().min(1),
  role: z.enum(['admin', 'editor', 'reader', 'system']),
  createdAt: z.string(),
  updatedAt: z.string(),
});

const FeatureFlagSchema = z.object({
  name: z.string().min(1),
  enabled: z.boolean(),
  description: z.string(),
  value: z.unknown(),
  updatedAt: z.string(),
});

function parseSelfEvolveState(raw: unknown): SelfEvolveState {
  const parsed = SelfEvolveStateSchema.safeParse(raw);
  if (!parsed.success) throw new ApiError('The server returned invalid self-evolve state.', { code: 'INVALID_RESPONSE' });
  const { status, cycleCount, lastRevisionHash, lastEvalScore } = parsed.data;
  const base = { status, cycleCount };
  if (lastRevisionHash === undefined && lastEvalScore === undefined) return base;
  if (lastRevisionHash === undefined && lastEvalScore !== undefined) return { ...base, lastEvalScore };
  if (lastRevisionHash !== undefined && lastEvalScore === undefined) return { ...base, lastRevisionHash };
  if (lastRevisionHash !== undefined && lastEvalScore !== undefined) return { ...base, lastRevisionHash, lastEvalScore };
  throw new ApiError('The server returned invalid self-evolve state.', { code: 'INVALID_RESPONSE' });
}

const AlertRuleSchema = z.object({
  id: z.string(),
  name: z.string(),
  type: z.string(),
  severity: z.enum(['info', 'warning', 'critical']),
  enabled: z.boolean(),
  threshold: z.number(),
  duration: z.number(),
  window: z.number(),
  config: z.string().nullable(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

const PreconditionSchema = z.object({
  id: z.string(),
  capabilityId: z.string(),
  name: z.string(),
  command: z.string(),
  timeoutSec: z.number(),
  enabled: z.boolean(),
  createdAt: z.string(),
  updatedAt: z.string().optional(),
});

const CapabilitySchema = z.object({
  id: z.string(),
  projectId: z.string(),
  name: z.string(),
  description: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
  selfEvolveEnabled: z.boolean(),
  selfEvolveMinScore: z.number().min(0).max(1),
  selfEvolveMaxRevisions: z.number().int().nonnegative(),
  selfEvolveCooldownSec: z.number().int().nonnegative(),
  selfEvolveTargetEnv: z.string(),
  selfEvolveDatasetId: z.string(),
});

const ProjectSchema = z.object({
  id: z.string(),
  workspaceId: z.string(),
  name: z.string(),
  description: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

const ReleaseSchema = z.object({
  id: z.string(),
  capabilityId: z.string(),
  capabilityVersion: z.number().int().positive(),
  capabilityVersionId: z.string().nullable(),
  manifest: z.string(),
  environment: z.enum(['dev', 'staging', 'prod']),
  status: z.enum(['draft', 'review', 'approved', 'canary', 'active', 'rolled_back']),
  approvedBy: z.string(),
  replacesReleaseId: z.string().nullable(),
  createdAt: z.string(),
  createdBy: z.string(),
  activatedAt: z.string().nullable(),
  canaryPercent: z.number().int().min(0).max(100),
  signature: z.string().nullable().optional(),
  signedKeyId: z.string().nullable().optional(),
  signedAt: z.string().nullable().optional(),
});

const CanaryAssessmentSchema = z.object({
  action: z.enum(['no_action', 'rolled_back']),
  reason: z.string(),
  releaseId: z.string(),
  targetReleaseId: z.string().optional(),
  evaluationId: z.string().optional(),
});

const ApprovalEntrySchema = z.object({
  userId: z.string(),
  vote: z.enum(['approve', 'reject']),
  comment: z.string(),
  createdAt: z.string(),
});

const ApprovalSummarySchema = z.object({
  releaseId: z.string(),
  manifestHash: z.string().optional(),
  distinctApprovers: z.number().int().nonnegative(),
  approvals: z.array(ApprovalEntrySchema),
});

const PendingApprovalSummarySchema = z.object({
  releaseId: z.string(),
  manifestHash: z.string(),
  approvals: z.array(ApprovalEntrySchema),
  updatedAt: z.string(),
});

function assertManifest(value: unknown): asserts value is Manifest {
  const parsed = ManifestSchema.safeParse(value);
  if (!parsed.success) throw new ApiError('The server returned invalid manifest data.', { code: 'INVALID_RESPONSE' });
}

const AuditEntrySchema = z.object({
  id: z.string(),
  userId: z.string(),
  action: z.string(),
  resource: z.string(),
  details: z.string(),
  timestamp: z.string(),
  entryHash: z.string(),
  resourceKind: z.string(),
  resourceId: z.string(),
});

const ExecutionSchema = z.object({
  id: z.string(),
  capabilityVersionId: z.string().nullable(),
  timestamp: z.string(),
  inputs: z.string(),
  outputs: z.string(),
  model: z.string(),
  provider: z.string(),
  latencyMs: z.number().nonnegative(),
  costUsd: z.number().nonnegative(),
  promptTokens: z.number().int().nonnegative(),
  completionTokens: z.number().int().nonnegative(),
  totalTokens: z.number().int().nonnegative(),
  error: z.string(),
  traceId: z.string(),
  environment: z.string(),
  replayOf: z.string().nullable(),
  replayCount: z.number().int().nonnegative(),
  inputHash: z.string().nullable(),
});

const ScheduleSchema = z.object({
  id: z.string(),
  workspaceId: z.string(),
  releaseId: z.string(),
  kind: z.string(),
  cron: z.string(),
  webhookPath: z.string(),
  nextFireAt: z.string(),
  lastFireAt: z.string().nullable(),
  firedCount: z.number().int().nonnegative(),
  enabled: z.boolean(),
  createdAt: z.string(),
  createdBy: z.string(),
});

const DatasetSchema = z.object({
  id: z.string(), capabilityId: z.string(), name: z.string(), description: z.string(),
  createdAt: z.string(), updatedAt: z.string(),
});
const DatasetCaseSchema = z.object({
  id: z.string(), datasetId: z.string(), seq: z.number().int().positive(), inputs: z.string(),
  expected: z.string(), description: z.string(),
});

function parseVaultKeyring(raw: unknown): VaultKeyringEntry[] {
  return unwrapList<unknown>(raw).map((entry) => {
    const parsed = VaultKeyringEntrySchema.safeParse(entry);
    if (!parsed.success) {
      throw new ApiError('The server returned an invalid vault keyring.', { code: 'INVALID_RESPONSE' });
    }
    return parsed.data;
  });
}

function parseCostRollups(raw: unknown): CostRollup[] {
  return unwrapList<unknown>(raw).map((entry) => {
    const parsed = CostRollupSchema.safeParse(entry);
    if (!parsed.success) {
      throw new ApiError('The server returned invalid cost rollup data.', { code: 'INVALID_RESPONSE' });
    }
    return parsed.data;
  });
}

function parseEvalSuites(raw: unknown): EvalSuite[] {
  return unwrapList<unknown>(raw).map((entry) => {
    const parsed = EvalSuiteSchema.safeParse(entry);
    if (!parsed.success) {
      throw new ApiError('The server returned invalid eval suite data.', { code: 'INVALID_RESPONSE' });
    }
    return parsed.data;
  });
}

function parseEvalSuiteDetail(raw: unknown): { suite: EvalSuite; versions: EvalSuiteVersion[] } {
  if (!raw || typeof raw !== 'object') {
    throw new ApiError('The server returned invalid eval suite details.', { code: 'INVALID_RESPONSE' });
  }
  const value = raw as Record<string, unknown>;
  const suite = EvalSuiteSchema.safeParse(value['suite']);
  const versions = z.array(EvalSuiteVersionSchema).safeParse(value['versions']);
  if (!suite.success || !versions.success) {
    throw new ApiError('The server returned invalid eval suite details.', { code: 'INVALID_RESPONSE' });
  }
  return { suite: suite.data, versions: versions.data };
}

function parseEvalSuiteRuns(raw: unknown): EvalSuiteRun[] {
  return unwrapList<unknown>(raw).map((entry) => {
    const parsed = EvalSuiteRunSchema.safeParse(entry);
    if (!parsed.success) throw new ApiError('The server returned invalid eval suite run data.', { code: 'INVALID_RESPONSE' });
    return parsed.data;
  });
}

function parseMutationProposals(raw: unknown): MutationProposal[] {
  const parsed = z.array(MutationProposalSchema).safeParse(unwrapList<unknown>(raw));
  if (!parsed.success) throw new ApiError('The server returned invalid mutation proposal data.', { code: 'INVALID_RESPONSE' });
  return parsed.data;
}

function parseEvalRun(raw: unknown): EvalRun {
  const parsed = EvalRunSchema.safeParse(raw);
  if (!parsed.success) {
    throw new ApiError('The server returned invalid eval run data.', { code: 'INVALID_RESPONSE' });
  }
  return parsed.data;
}

function parseEvalRuns(raw: unknown): EvalRun[] {
  return unwrapList<unknown>(raw).map(parseEvalRun);
}

function parseEvalResults(raw: unknown): EvalResult[] {
  return unwrapList<unknown>(raw).map((entry) => {
    const parsed = EvalResultSchema.safeParse(entry);
    if (!parsed.success) {
      throw new ApiError('The server returned invalid eval result data.', { code: 'INVALID_RESPONSE' });
    }
    return parsed.data;
  });
}

function parseMergeRequest(raw: unknown): MergeRequest {
  const parsed = MergeRequestSchema.safeParse(raw);
  if (!parsed.success) {
    throw new ApiError('The server returned invalid merge request data.', { code: 'INVALID_RESPONSE' });
  }
  return parsed.data;
}

function parseMergeRequests(raw: unknown): MergeRequest[] {
  return unwrapList<unknown>(raw).map(parseMergeRequest);
}

function parseMergeRequestDetail(raw: unknown): {
  mr: MergeRequest;
  approvals: MergeRequestApproval[];
  comments: MergeRequestComment[];
} {
  if (!raw || typeof raw !== 'object') {
    throw new ApiError('The server returned invalid merge request details.', { code: 'INVALID_RESPONSE' });
  }
  const value = raw as Record<string, unknown>;
  const mr = parseMergeRequest(value['mr']);
  const approvals = Array.isArray(value['approvals']) ? value['approvals'].map((entry) => {
    const parsed = MergeRequestApprovalSchema.safeParse(entry);
    if (!parsed.success) throw new ApiError('The server returned invalid merge request approvals.', { code: 'INVALID_RESPONSE' });
    return parsed.data;
  }) : null;
  const comments = Array.isArray(value['comments']) ? value['comments'].map((entry) => {
    const parsed = MergeRequestCommentSchema.safeParse(entry);
    if (!parsed.success) throw new ApiError('The server returned invalid merge request comments.', { code: 'INVALID_RESPONSE' });
    return parsed.data;
  }) : null;
  if (!approvals || !comments) {
    throw new ApiError('The server returned invalid merge request details.', { code: 'INVALID_RESPONSE' });
  }
  return { mr, approvals, comments };
}

function parseCapabilityVersions(raw: unknown): CapabilityVersion[] {
  return unwrapList<unknown>(raw).map((entry) => {
    const parsed = CapabilityVersionSchema.safeParse(entry);
    if (!parsed.success) {
      throw new ApiError('The server returned invalid capability version data.', { code: 'INVALID_RESPONSE' });
    }
    return parsed.data;
  });
}

function parseCapabilityManifest(raw: unknown): CapabilityManifestResponse {
  const parsed = CapabilityManifestResponseSchema.safeParse(raw);
  if (!parsed.success) {
    throw new ApiError('The server returned invalid capability manifest data.', { code: 'INVALID_RESPONSE' });
  }
  return parsed.data;
}

function parseSearchResults(raw: unknown): SearchResult[] {
  const parsed = z.array(SearchResultSchema).safeParse(unwrapList<unknown>(raw));
  if (!parsed.success) throw new ApiError('The server returned invalid search results.', { code: 'INVALID_RESPONSE' });
  return parsed.data;
}

function parseAlerts(raw: unknown): Alert[] {
  const parsed = z.array(AlertSchema).safeParse(unwrapList<unknown>(raw));
  if (!parsed.success) throw new ApiError('The server returned invalid alert data.', { code: 'INVALID_RESPONSE' });
  return parsed.data;
}

function parseSettings(raw: unknown): SettingItem[] {
  const parsed = z.array(SettingItemSchema).safeParse(unwrapList<unknown>(raw));
  if (!parsed.success) throw new ApiError('The server returned invalid settings data.', { code: 'INVALID_RESPONSE' });
  return parsed.data;
}

function parseAlertRules(raw: unknown): AlertRule[] {
  const parsed = z.array(AlertRuleSchema).safeParse(unwrapList<unknown>(raw));
  if (!parsed.success) throw new ApiError('The server returned invalid alert rule data.', { code: 'INVALID_RESPONSE' });
  return parsed.data;
}

function parsePreconditions(raw: unknown): Precondition[] {
  const parsed = z.array(PreconditionSchema).safeParse(unwrapList<unknown>(raw));
  if (!parsed.success) throw new ApiError('The server returned invalid precondition data.', { code: 'INVALID_RESPONSE' });
  return parsed.data.map((value) => {
    const base = {
      id: value.id,
      capabilityId: value.capabilityId,
      name: value.name,
      command: value.command,
      timeoutSec: value.timeoutSec,
      enabled: value.enabled,
      createdAt: value.createdAt,
    };
    return value.updatedAt === undefined ? base : { ...base, updatedAt: value.updatedAt };
  });
}

function parseCapabilities(raw: unknown): Capability[] {
  const parsed = z.array(CapabilitySchema).safeParse(unwrapList<unknown>(raw));
  if (!parsed.success) throw new ApiError('The server returned invalid capability data.', { code: 'INVALID_RESPONSE' });
  return parsed.data;
}

function parseProjects(raw: unknown): Project[] {
  const parsed = z.array(ProjectSchema).safeParse(unwrapList<unknown>(raw));
  if (!parsed.success) throw new ApiError('The server returned invalid project data.', { code: 'INVALID_RESPONSE' });
  return parsed.data;
}

function parseReleases(raw: unknown): Release[] {
  const parsed = z.array(ReleaseSchema).safeParse(unwrapList<unknown>(raw));
  if (!parsed.success) throw new ApiError('The server returned invalid release data.', { code: 'INVALID_RESPONSE' });
  return parsed.data;
}

function parseAuditEntries(raw: unknown): AuditEntry[] {
  const value = raw && typeof raw === 'object' ? raw as Record<string, unknown> : undefined;
  const entries = value?.['entries'];
  const parsed = z.array(AuditEntrySchema).safeParse(entries);
  if (!parsed.success) throw new ApiError('The server returned invalid audit data.', { code: 'INVALID_RESPONSE' });
  return parsed.data;
}

function parseExecution(raw: unknown): Execution {
  const parsed = ExecutionSchema.safeParse(raw);
  if (!parsed.success) throw new ApiError('The server returned invalid execution data.', { code: 'INVALID_RESPONSE' });
  return parsed.data;
}

function parseExecutionPage(raw: unknown): { items: Execution[]; total: number } {
  if (!raw || typeof raw !== 'object') throw new ApiError('The server returned invalid execution data.', { code: 'INVALID_RESPONSE' });
  const value = raw as Record<string, unknown>;
  const items = Array.isArray(value['items']) ? value['items'].map(parseExecution) : null;
  const total = value['total'];
  if (!items || typeof total !== 'number' || !Number.isInteger(total) || total < 0) {
    throw new ApiError('The server returned invalid execution data.', { code: 'INVALID_RESPONSE' });
  }
  return { items, total };
}

function parseSchedule(raw: unknown): Schedule {
  const parsed = ScheduleSchema.safeParse(raw);
  if (!parsed.success) throw new ApiError('The server returned invalid schedule data.', { code: 'INVALID_RESPONSE' });
  return parsed.data;
}

function parseSchedulePage(raw: unknown): { items: Schedule[]; total: number } {
  if (!raw || typeof raw !== 'object') throw new ApiError('The server returned invalid schedule data.', { code: 'INVALID_RESPONSE' });
  const value = raw as Record<string, unknown>;
  const items = Array.isArray(value['items']) ? value['items'].map(parseSchedule) : null;
  const total = value['total'];
  if (!items || typeof total !== 'number' || !Number.isInteger(total) || total < 0) {
    throw new ApiError('The server returned invalid schedule data.', { code: 'INVALID_RESPONSE' });
  }
  return { items, total };
}

function parseRelease(raw: unknown): Release {
  const parsed = ReleaseSchema.safeParse(raw);
  if (!parsed.success) throw new ApiError('The server returned invalid release data.', { code: 'INVALID_RESPONSE' });
  return parsed.data;
}

const WorkspaceRowSchema = z.object({
  id: z.string().min(1),
  name: z.string(),
  organization: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

function parseWorkspace(raw: unknown): WorkspaceRow {
  const parsed = WorkspaceRowSchema.safeParse(raw);
  if (!parsed.success) {
    throw new ApiError('The server returned an invalid workspace.', { code: 'INVALID_RESPONSE' });
  }
  return parsed.data;
}

function parseWorkspaceList(raw: unknown): WorkspaceRow[] {
  return unwrapList<unknown>(raw).map(parseWorkspace);
}

/**
 * Backend list endpoints come back in two shapes:
 *
 *   - Bare array:                    GET /api/...
 *   - { items: T[], total: number }  GET /api/...?page=N
 *   - { <noun>s: T[] }               GET /api/...   (e.g. {webhooks, flags, keys})
 *
 * Many pages today write `unwrapArray(unknownResponse)` inline.
 * The unwrapList helper centralizes that.
 */
export function unwrapList<T>(raw: unknown, pluralKey?: string): T[] {
  if (Array.isArray(raw)) return raw as T[];
  if (raw && typeof raw === 'object') {
    const obj = raw as Record<string, unknown>;
    if (Array.isArray(obj['items'])) return obj['items'] as T[];
    if (Array.isArray(obj['results'])) return obj['results'] as T[];
    if (pluralKey && Array.isArray(obj[pluralKey])) return obj[pluralKey] as T[];
    // Heuristic fallback: pick the first array-valued key.
    for (const k of Object.keys(obj)) {
      const v = obj[k];
      if (Array.isArray(v)) return v as T[];
    }
  }
  return [];
}

/** Parse a list-shaped API response at the browser/server boundary. */
export function parseList<T>(raw: unknown, schema: z.ZodType<T>, pluralKey?: string): T[] {
  const parsed = z.array(schema).safeParse(unwrapList<unknown>(raw, pluralKey));
  if (!parsed.success) {
    throw new ApiError('The server returned invalid list data.', { code: 'INVALID_RESPONSE' });
  }
  return parsed.data;
}

export function unwrapFirst<T>(raw: unknown, pluralKey?: string): T | null {
  const items = unwrapList<T>(raw, pluralKey);
  return items[0] ?? null;
}

export function subscribeSSE(channel: string, onEvent: (event: unknown) => void): () => void {
  if (typeof window === 'undefined') return () => undefined;
  const encodedChannel = encodeURIComponent(channel.trim());
  if (encodedChannel === '') return () => undefined;
  let cancelled = false;
  let active: EventSource | null = null;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  const open = () => {
    if (cancelled) return;
    active = new EventSource(`/api/events/${encodedChannel}`);
    active.onmessage = (e) => {
      let payload: unknown;
      try {
        payload = JSON.parse(e.data) as unknown;
      } catch {
        payload = { raw: e.data };
      }
      onEvent(payload);
    };
    active.onerror = () => {
      active?.close();
      active = null;
      if (cancelled) return;
      reconnectTimer = setTimeout(open, 3000);
    };
  };

  open();

  return () => {
    cancelled = true;
    if (reconnectTimer) clearTimeout(reconnectTimer);
    active?.close();
    active = null;
  };
}

export const workspaceApi = {
  list: async (page = 1, pageSize = 100): Promise<{ data: WorkspaceRow[] }> => {
    const r = await client.get<unknown>('/workspaces', {
      params: { page, pageSize },
    });
    return { data: parseWorkspaceList(r.data) };
  },
  get: (id: string): Promise<{ data: WorkspaceRow }> =>
    client.get<unknown>(`/workspaces/${id}`).then((r) => ({ data: parseWorkspace(r.data) })),
  create: (data: { name: string; organization?: string }): Promise<{ data: WorkspaceRow }> =>
    client.post<unknown>('/workspaces', data).then((r) => ({ data: parseWorkspace(r.data) })),
  update: (id: string, data: { name?: string; organization?: string }): Promise<{ data: WorkspaceRow }> =>
    client.put<unknown>(`/workspaces/${id}`, data).then((r) => ({ data: parseWorkspace(r.data) })),
  delete: (id: string) => client.delete(`/workspaces/${id}`),
};

const AgentSpecificationMetadataSchema = z.object({
  hash: z.string().regex(/^[0-9a-f]{64}$/),
  workspaceId: z.string(),
  schemaVersion: z.string(),
  parentHash: z.string().regex(/^[0-9a-f]{64}$/).nullable(),
  author: z.string(),
  changeReason: z.string(),
  status: z.enum(['draft', 'candidate', 'published', 'retired']),
  createdAt: z.string(),
  publishedAt: z.string().nullable(),
});

const AgentSpecificationListSchema = z.object({
  items: z.array(AgentSpecificationMetadataSchema),
  total: z.number().int().nonnegative(),
});

const AgentSpecificationSecurityFindingSchema = z.object({
  rule: z.string(),
  severity: z.enum(['info', 'warn', 'block']),
  message: z.string(),
  range: z.object({ start: z.number().int().nonnegative(), end: z.number().int().nonnegative() }).optional(),
  snippet: z.string().optional(),
});
const AgentSpecificationSecuritySchema = z.object({
  verdict: z.enum(['clean', 'warn', 'block']),
  findings: z.array(AgentSpecificationSecurityFindingSchema),
});
const AgentSpecificationRecordSchema = AgentSpecificationMetadataSchema.extend({
  specification: z.unknown(),
  security: AgentSpecificationSecuritySchema.optional(),
});

function parseAgentSpecificationList(raw: unknown): AgentSpecificationList {
  const parsed = AgentSpecificationListSchema.safeParse(raw);
  if (!parsed.success) throw new ApiError('The server returned invalid agent specification data.', { code: 'INVALID_RESPONSE' });
  return parsed.data;
}

function parseAgentSpecificationRecord(raw: unknown): AgentSpecificationRecord {
  const parsed = AgentSpecificationRecordSchema.safeParse(raw);
  if (!parsed.success) throw new ApiError('The server returned invalid agent revision data.', { code: 'INVALID_RESPONSE' });
  return parsed.data;
}

export const agentSpecificationApi = {
  list: async (
    workspaceId: string,
    options: { page?: number; pageSize?: number; status?: AgentSpecificationStatus } = {},
  ): Promise<{ data: AgentSpecificationList }> => {
    const response = await client.get<unknown>(`/workspaces/${encodeURIComponent(workspaceId)}/agent-specifications`, {
      params: options,
    });
    return { data: parseAgentSpecificationList(response.data) };
  },
  create: async (data: {
    workspaceId: string;
    specification: AgentSpecificationDraft;
    changeReason: string;
    parentHash?: string;
  }): Promise<{ data: AgentSpecificationRecord }> => {
    const response = await client.post<unknown>(`/workspaces/${encodeURIComponent(data.workspaceId)}/agent-specifications`, {
      specification: data.specification,
      changeReason: data.changeReason,
      ...(data.parentHash ? { parentHash: data.parentHash } : {}),
    });
    return { data: parseAgentSpecificationRecord(response.data) };
  },
  validate: async (workspaceId: string, specification: AgentSpecificationDraft): Promise<{ data: { valid: true; specification: unknown; security: AgentSpecificationSecurityReport } | { valid: false; issues: AgentSpecificationValidationIssue[]; security?: AgentSpecificationSecurityReport | undefined } }> => {
    const response = await client.post<unknown>(`/workspaces/${encodeURIComponent(workspaceId)}/agent-specifications/validate`, { specification });
    const parsed = z.union([
      z.object({ valid: z.literal(true), specification: z.unknown(), security: AgentSpecificationSecuritySchema }),
      z.object({
        valid: z.literal(false),
        issues: z.array(z.object({ code: z.string(), message: z.string(), path: z.array(z.union([z.string(), z.number()])) })),
        security: AgentSpecificationSecuritySchema.optional(),
      }),
    ]).safeParse(response.data);
    if (!parsed.success) throw new ApiError('The server returned invalid validation data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  get: async (workspaceId: string, hash: string): Promise<{ data: AgentSpecificationRecord }> => {
    const response = await client.get<unknown>(`/workspaces/${encodeURIComponent(workspaceId)}/agent-specifications/${encodeURIComponent(hash)}`);
    return { data: parseAgentSpecificationRecord(response.data) };
  },
  lineage: async (workspaceId: string, hash: string): Promise<{ data: AgentSpecificationMetadata[] }> => {
    const response = await client.get<unknown>(`/workspaces/${encodeURIComponent(workspaceId)}/agent-specifications/${encodeURIComponent(hash)}/lineage`);
    const parsed = z.object({ items: z.array(AgentSpecificationMetadataSchema) }).safeParse(response.data);
    if (!parsed.success) throw new ApiError('The server returned invalid agent lineage data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data.items };
  },
  diff: async (workspaceId: string, leftHash: string, rightHash: string): Promise<{ data: AgentSpecificationDiffEntry[] }> => {
    const response = await client.post<unknown>(`/workspaces/${encodeURIComponent(workspaceId)}/agent-specifications/diff`, { leftHash, rightHash });
    const parsed = z.object({ changes: z.array(z.object({ path: z.string(), before: z.unknown(), after: z.unknown() })) }).safeParse(response.data);
    if (!parsed.success) throw new ApiError('The server returned invalid agent diff data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data.changes };
  },
  publish: async (workspaceId: string, hash: string): Promise<{ data: AgentSpecificationRecord }> => {
    const response = await client.post<unknown>(`/workspaces/${encodeURIComponent(workspaceId)}/agent-specifications/${encodeURIComponent(hash)}/publish`);
    return { data: parseAgentSpecificationRecord(response.data) };
  },
};

export const projectApi = {
  list: async (workspaceId: string): Promise<{ data: Project[] }> => {
    const r = await client.get<unknown>('/projects', { params: { workspaceId } });
    return { data: parseProjects(r.data) };
  },
  get: async (id: string): Promise<{ data: Project }> => {
    const r = await client.get<unknown>(`/projects/${id}`);
    const parsed = ProjectSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid project data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  create: (data: { workspaceId: string; name: string; description?: string }) => client.post('/projects', data),
  update: (id: string, data: { name?: string; description?: string }) => client.put(`/projects/${id}`, data),
  delete: (id: string) => client.delete(`/projects/${id}`),
};

export const capabilityApi = {
  list: async (projectId: string): Promise<{ data: Capability[] }> => {
    const r = await client.get<unknown>('/capabilities', { params: { projectId } });
    return { data: parseCapabilities(r.data) };
  },
  get: async (id: string): Promise<{ data: Capability }> => {
    const r = await client.get<unknown>(`/capabilities/${id}`);
    const parsed = CapabilitySchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid capability data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  create: async (data: { projectId: string; name: string; description?: string }): Promise<{ data: Capability }> => {
    const r = await client.post<unknown>('/capabilities', data);
    const parsed = CapabilitySchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid capability data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  update: (id: string, data: { name?: string; description?: string }) => client.put(`/capabilities/${id}`, data),
  delete: (id: string) => client.delete(`/capabilities/${id}`),
};

export const versionApi = {
  list: async (capabilityId: string): Promise<{ data: CapabilityVersion[] }> => {
    const r = await client.get<unknown>('/capability-versions', { params: { capabilityId } });
    return { data: parseCapabilityVersions(r.data) };
  },
  get: async (id: string): Promise<{ data: CapabilityVersion }> => {
    const r = await client.get<unknown>(`/capability-versions/${id}`);
    const parsed = CapabilityVersionSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid capability version data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  create: (data: { capabilityId: string; version: number; manifest: string; manifestHash: string; createdBy?: string }) =>
    client.post('/capability-versions', data),
};

export const releaseApi = {
  list: async (capabilityId: string): Promise<{ data: Release[] }> => {
    const r = await client.get<unknown>('/releases', { params: { capabilityId } });
    return { data: parseReleases(r.data) };
  },
  listAll: async (page = 1, pageSize = 100): Promise<{ data: { items: Release[]; total: number } }> => {
    const r = await client.get<unknown>('/releases', { params: { page, pageSize } });
    if (!r.data || typeof r.data !== 'object') throw new ApiError('The server returned invalid release data.', { code: 'INVALID_RESPONSE' });
    const value = r.data as Record<string, unknown>;
    const items = parseReleases(value['items']);
    if (typeof value['total'] !== 'number' || !Number.isInteger(value['total']) || value['total'] < 0) {
      throw new ApiError('The server returned invalid release totals.', { code: 'INVALID_RESPONSE' });
    }
    return { data: { items, total: value['total'] } };
  },
  get: async (id: string): Promise<{ data: Release }> => {
    const r = await client.get<unknown>(`/releases/${id}`);
    return { data: parseRelease(r.data) };
  },
  create: (data: { capabilityId: string; capabilityVersion: number; capabilityVersionId: string | null; manifest: string; environment: string }) =>
    client.post('/releases', data),
  sign: (id: string, data: { keyId: string; signature: string; signedAt: string }) =>
    client.post(`/releases/${id}/sign`, data),
  transition: (id: string, to: 'draft' | 'review' | 'approved' | 'canary' | 'active' | 'rolled_back', reason?: string) =>
    client.post(`/releases/${id}/transition`, { to, ...(reason ? { reason } : {}) }),
  canary: (id: string, percent: number) => client.put(`/releases/${id}/canary`, { percent }),
  rollback: (id: string, toReleaseId?: string) => {
    const body: { toReleaseId?: string } = {};
    if (toReleaseId !== undefined) body.toReleaseId = toReleaseId;
    return client.post(`/releases/${id}/rollback`, body);
  },
  autoRollback: async (id: string): Promise<{ action: 'no_action' | 'rolled_back'; reason: string; releaseId: string; targetReleaseId?: string | undefined; evaluationId?: string | undefined }> => {
    const r = await client.post<unknown>(`/releases/${id}/auto-rollback`, {});
    const parsed = CanaryAssessmentSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid canary assessment data.', { code: 'INVALID_RESPONSE' });
    return parsed.data;
  },
};

export const executionApi = {
  list: async (capabilityVersionId: string): Promise<{ data: { items: Execution[]; total: number } }> => {
    const r = await client.get<unknown>('/executions', { params: { capabilityVersionId } });
    return { data: parseExecutionPage(r.data) };
  },
  get: async (id: string): Promise<{ data: Execution }> => {
    const r = await client.get<unknown>(`/executions/${id}`);
    return { data: parseExecution(r.data) };
  },
  execute: (data: { manifestHash: string; inputs: Record<string, unknown>; environment?: string; traceId?: string; preview?: boolean }) =>
    client.post('/executions', data, { timeout: 130_000 }),
  replay: (id: string) => client.post(`/executions/${id}/replay`),
  replays: (id: string) => client.get(`/executions/${id}/replays`),
  /**
   * Open a server-sent event connection to a streaming execution.
   * Returns an `AbortController` so the caller can cancel.
   */
  stream: (
    data: { manifestHash: string; inputs: Record<string, unknown>; environment?: string; traceId?: string },
    onFrame: (frame: { event: string; data: Record<string, unknown>; timestamp: string }) => void,
  ): AbortController => {
    const controller = new AbortController();
    const base = baseURL();
    const url = `${base}/api/executions`;
    const headers: Record<string, string> = {
      'content-type': 'application/json',
      accept: 'text/event-stream',
    };
    void fetch(url, {
      method: 'POST',
      headers,
      credentials: 'same-origin',
      body: JSON.stringify(data),
      signal: controller.signal,
    }).then(async (res) => {
      if (!res.ok || !res.body) return;
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';
      while (true) {
        const { value, done } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        let idx: number;
        while ((idx = buffer.indexOf('\n\n')) !== -1) {
          const block = buffer.slice(0, idx);
          buffer = buffer.slice(idx + 2);
          const frame = parseSseBlock(block);
          if (frame) onFrame(frame);
        }
      }
    }).catch(() => undefined);
    return controller;
  },
};

export interface ExecutionQueueMetrics {
  queued: number;
  running: number;
  completed: number;
  failed: number;
  cancelled: number;
  timedOut: number;
  partiallyCompleted: number;
  oldestQueuedAt: string | null;
}

export type ExecutionJobState = 'queued' | 'running' | 'completed' | 'failed' | 'cancelled' | 'timed-out' | 'partially-completed';

export interface ExecutionJob {
  id: string;
  organizationId: string;
  workspaceId: string;
  agentHash: string;
  inputHash: string;
  inputJson: string;
  idempotencyKey: string;
  state: ExecutionJobState;
  attempts: number;
  maxAttempts: number;
  availableAt: string;
  leaseOwner: string | null;
  leaseExpiresAt: string | null;
  resultJson: string | null;
  error: string | null;
  createdAt: string;
  startedAt: string | null;
  completedAt: string | null;
}

const ExecutionJobSchema = z.object({
  id: z.string().uuid(),
  organizationId: z.string(),
  workspaceId: z.string(),
  agentHash: z.string().regex(/^[0-9a-f]{64}$/),
  inputHash: z.string().regex(/^[0-9a-f]{64}$/),
  inputJson: z.string(),
  idempotencyKey: z.string(),
  state: z.enum(['queued', 'running', 'completed', 'failed', 'cancelled', 'timed-out', 'partially-completed']),
  attempts: z.number().int().nonnegative(),
  maxAttempts: z.number().int().positive(),
  availableAt: z.string(),
  leaseOwner: z.string().nullable(),
  leaseExpiresAt: z.string().nullable(),
  resultJson: z.string().nullable(),
  error: z.string().nullable(),
  createdAt: z.string(),
  startedAt: z.string().nullable(),
  completedAt: z.string().nullable(),
});

function parseExecutionJob(raw: unknown): ExecutionJob {
  const parsed = ExecutionJobSchema.safeParse(raw);
  if (!parsed.success) throw new ApiError('The server returned invalid execution job data.', { code: 'INVALID_RESPONSE' });
  return parsed.data;
}

export interface HealthStatus {
  status: 'ok' | 'error';
  db: 'ok' | 'error';
  timestamp: string;
  gateway?: {
    cacheEntries: number;
    cacheHits: number;
    cacheMisses: number;
    rateLimiter: {
      activeBuckets: number;
      maxBuckets: number;
      totalRequests: number;
      deniedRequests: number;
      bucketEvictions: number;
    };
  } | undefined;
}

export const healthApi = {
  status: async (): Promise<{ data: HealthStatus }> => {
    const r = await client.get<unknown>('/health');
    return { data: z.object({
      status: z.union([z.literal('ok'), z.literal('error')]),
      db: z.union([z.literal('ok'), z.literal('error')]),
      timestamp: z.string(),
      gateway: z.object({
        cacheEntries: z.number().int().nonnegative(),
        cacheHits: z.number().int().nonnegative(),
        cacheMisses: z.number().int().nonnegative(),
        rateLimiter: z.object({
          activeBuckets: z.number().int().nonnegative(),
          maxBuckets: z.number().int().positive(),
          totalRequests: z.number().int().nonnegative(),
          deniedRequests: z.number().int().nonnegative(),
          bucketEvictions: z.number().int().nonnegative(),
        }),
      }).optional(),
    }).parse(r.data) };
  },
};

export const executionJobApi = {
  enqueue: async (workspaceId: string, data: { agentHash: string; inputs: Record<string, unknown>; idempotencyKey: string; maxAttempts?: number }): Promise<{ data: ExecutionJob }> => {
    const r = await client.post<unknown>(`/workspaces/${encodeURIComponent(workspaceId)}/execution-jobs`, data);
    return { data: parseExecutionJob(r.data) };
  },
  get: async (workspaceId: string, id: string): Promise<{ data: ExecutionJob }> => {
    const r = await client.get<unknown>(`/workspaces/${encodeURIComponent(workspaceId)}/execution-jobs/${encodeURIComponent(id)}`);
    return { data: parseExecutionJob(r.data) };
  },
  cancel: async (workspaceId: string, id: string): Promise<{ data: ExecutionJob }> => {
    const r = await client.post<unknown>(`/workspaces/${encodeURIComponent(workspaceId)}/execution-jobs/${encodeURIComponent(id)}/cancel`);
    return { data: parseExecutionJob(r.data) };
  },
  metrics: async (): Promise<{ data: ExecutionQueueMetrics }> => {
    const r = await client.get<unknown>('/execution-jobs/metrics');
    return { data: z.object({
      queued: z.number().int().nonnegative(),
      running: z.number().int().nonnegative(),
      completed: z.number().int().nonnegative(),
      failed: z.number().int().nonnegative(),
      cancelled: z.number().int().nonnegative(),
      timedOut: z.number().int().nonnegative(),
      partiallyCompleted: z.number().int().nonnegative(),
      oldestQueuedAt: z.string().nullable(),
    }).parse(r.data) };
  },
};

function baseURL(): string {
  if (typeof window !== 'undefined') return '';
  return process.env['NEXT_PUBLIC_API_BASE'] ?? '';
}

function parseSseBlock(block: string): { event: string; data: Record<string, unknown>; timestamp: string } | null {
  const lines = block.split('\n');
  let event = '';
  let data = '';
  for (const line of lines) {
    if (line.startsWith('event: ')) event = line.slice(7).trim();
    else if (line.startsWith('data: ')) data += line.slice(6);
  }
  if (!event || !data) return null;
  let parsed: Record<string, unknown> = {};
  try {
    parsed = JSON.parse(data) as Record<string, unknown>;
  } catch {
    parsed = { raw: data };
  }
  return { event, data: parsed, timestamp: '' };
}

export const invokeApi = {
  // Use the canonical manifest-driven path for in-product calls.
  execute: (data: { manifestHash: string; inputs: Record<string, unknown>; environment?: string; traceId?: string }) =>
    client.post('/executions', data),
};

export const datasetApi = {
  list: async (capabilityId: string): Promise<{ data: Dataset[] }> => {
    const r = await client.get<unknown>('/datasets', { params: { capabilityId } });
    const parsed = z.array(DatasetSchema).safeParse(unwrapList<unknown>(r.data));
    if (!parsed.success) throw new ApiError('The server returned invalid dataset data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  get: async (id: string): Promise<{ data: Dataset }> => {
    const r = await client.get<unknown>(`/datasets/${id}`);
    const parsed = DatasetSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid dataset data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  create: async (data: { capabilityId: string; name: string; description?: string }): Promise<{ data: Dataset }> => {
    const r = await client.post<unknown>('/datasets', data);
    const parsed = DatasetSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid dataset data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  delete: (id: string) => client.delete(`/datasets/${id}`),
  getCases: async (id: string): Promise<{ data: DatasetCase[] }> => {
    const r = await client.get<unknown>(`/datasets/${id}/cases`);
    const parsed = z.array(DatasetCaseSchema).safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid dataset cases.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  addCase: async (id: string, data: { inputs: string; expected: string; description?: string }): Promise<{ data: DatasetCase }> => {
    const r = await client.post<unknown>(`/datasets/${id}/cases`, data);
    const parsed = DatasetCaseSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid dataset case data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
};

export const evalApi = {
  list: async (releaseId?: string): Promise<{ data: EvalRun[] }> => {
    const r = await client.get<unknown>('/eval-runs', { params: { releaseId } });
    return { data: parseEvalRuns(r.data) };
  },
  get: async (id: string): Promise<{ data: EvalRun }> => {
    const r = await client.get<unknown>(`/eval-runs/${id}`);
    return { data: parseEvalRun(r.data) };
  },
  create: async (data: { releaseId: string; datasetId: string; scorer: string }): Promise<{ data: EvalRun }> => {
    const r = await client.post<unknown>('/eval-runs', data);
    return { data: parseEvalRun(r.data) };
  },
  run: async (data: { evalRunId: string; getActualUrl: string }): Promise<{ data: EvalRun }> => {
    const r = await client.post<unknown>('/eval/run', data, { timeout: 130_000 });
    return { data: parseEvalRun(r.data) };
  },
  evaluators: async (): Promise<{ data: string[] }> => {
    const r = await client.get<unknown>('/eval/evaluators');
    const parsed = z.object({ evaluators: z.array(z.string().min(1)) }).safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid evaluator data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data.evaluators };
  },
  getResults: async (id: string): Promise<{ data: EvalResult[] }> => {
    const r = await client.get<unknown>(`/eval-runs/${id}/results`);
    return { data: parseEvalResults(r.data) };
  },
};

export const alertApi = {
  listRules: async (): Promise<{ data: AlertRule[] }> => {
    const r = await client.get<unknown>('/alert-rules');
    return { data: parseAlertRules(r.data) };
  },
  createRule: (data: { name: string; type: string; severity: string; threshold?: number; window?: number }) =>
    client.post('/alert-rules', data),
  deleteRule: (id: string) => client.delete(`/alert-rules/${id}`),
  listAlerts: async (): Promise<{ data: Alert[] }> => {
    const r = await client.get<unknown>('/alerts');
    return { data: parseAlerts(r.data) };
  },
  acknowledge: (id: string) => client.put(`/alerts/${id}/acknowledge`),
};

export const scheduleApi = {
  list: async (): Promise<{ data: { items: Schedule[]; total: number } }> => {
    const r = await client.get<unknown>('/schedules');
    return { data: parseSchedulePage(r.data) };
  },
  get: async (id: string): Promise<{ data: Schedule }> => {
    const r = await client.get<unknown>(`/schedules/${id}`);
    return { data: parseSchedule(r.data) };
  },
  create: async (data: { workspaceId: string; releaseId: string; kind: string; cron: string }): Promise<{ data: Schedule }> => {
    const r = await client.post<unknown>('/schedules', data);
    return { data: parseSchedule(r.data) };
  },
  delete: (id: string) => client.delete(`/schedules/${id}`),
};

export const settingsApi = {
  list: async (): Promise<{ data: SettingItem[] }> => {
    const r = await client.get<unknown>('/settings');
    return { data: parseSettings(r.data) };
  },
  get: async (key: string): Promise<{ data: SettingItem }> => {
    const r = await client.get<unknown>(`/settings/${encodeURIComponent(key)}`);
    const parsed = SettingItemSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid setting data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  set: async (key: string, value: unknown): Promise<{ data: SettingItem }> => {
    const r = await client.put<unknown>(`/settings/${encodeURIComponent(key)}`, { value });
    const parsed = SettingItemSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid setting data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
};

export const preconditionApi = {
  list: async (capabilityVersionId: string): Promise<{ data: Precondition[] }> => {
    const r = await client.get<unknown>('/preconditions', { params: { capabilityVersionId } });
    return { data: parsePreconditions(r.data) };
  },
  create: (data: { capabilityVersionId: string; name: string; command: string; enabled?: boolean }) =>
    client.post('/preconditions', data),
  update: (id: string, data: { name?: string; command?: string; enabled?: boolean }) => client.put(`/preconditions/${id}`, data),
  delete: (id: string) => client.delete(`/preconditions/${id}`),
};

export const approvalApi = {
  list: async (releaseId: string): Promise<{ data: ApprovalSummary }> => {
    const r = await client.get<unknown>('/approvals', { params: { releaseId } });
    const parsed = ApprovalSummarySchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid approval data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  listPending: async (): Promise<{ data: { approvals: PendingApprovalSummary[] } }> => {
    const r = await client.get<unknown>('/approvals/pending');
    if (!r.data || typeof r.data !== 'object') throw new ApiError('The server returned invalid pending approvals.', { code: 'INVALID_RESPONSE' });
    const parsed = z.object({ approvals: z.array(PendingApprovalSummarySchema) }).safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid pending approvals.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  vote: (releaseId: string, data: { decision: 'approve' | 'reject'; comment?: string }) =>
    client.post(`/releases/${releaseId}/approvals`, data),
};

export const compilerApi = {
  compile: (manifest: unknown, options?: { capabilityContext?: string; constraints?: string[] }) =>
    client.post('/compiler/compile', { manifest, ...options }),
  decompile: (manifest: unknown) => client.post('/compiler/decompile', { manifest }),
};

export const selfEvolveApi = {
  getState: async (capabilityId: string): Promise<{ data: SelfEvolveState }> => {
    const r = await client.get<unknown>(`/capabilities/${capabilityId}/self-evolve`);
    return { data: parseSelfEvolveState(r.data) };
  },
  runCycle: async (capabilityId: string): Promise<{ data: SelfEvolveCycle }> => {
    const r = await client.post<unknown>(`/capabilities/${capabilityId}/self-evolve/run`);
    const parsed = SelfEvolveCycleSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid self-evolve cycle data.', { code: 'INVALID_RESPONSE' });
    return { data: { action: parsed.data.action, state: parseSelfEvolveState(parsed.data.state) } };
  },
};

export const goalsApi = {
  list: async (limit = 20): Promise<{ data: { goals: GoalSummary[] } }> => {
    const r = await client.get<unknown>('/goals', { params: { limit } });
    const parsed = z.object({ goals: z.array(GoalSummarySchema) }).safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid goal summaries.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  get: async (hash: string): Promise<{ data: GoalDetail }> => {
    const r = await client.get<unknown>(`/goals/${encodeURIComponent(hash)}`);
    const parsed = GoalDetailSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid goal details.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
};

export const securityApi = {
  summary: async (days = 30): Promise<{ data: SecurityScanSummary }> => {
    const r = await client.get<unknown>('/security/scans/summary', { params: { days } });
    const parsed = SecurityScanSummarySchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid security summary data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  scan: async (text: string): Promise<{ data: SecurityScanResult }> => {
    const r = await client.post<unknown>('/security/scan', { text });
    const parsed = SecurityScanResultSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid security scan data.', { code: 'INVALID_RESPONSE' });
    return {
      data: {
        verdict: parsed.data.verdict,
        findings: parsed.data.findings.map((finding) => ({
          rule: finding.rule,
          severity: finding.severity,
          message: finding.message,
          snippet: finding.snippet ?? null,
          range: finding.range ?? null,
        })),
      },
    };
  },
};

export const manifestApi = {
  get: async (versionId: string): Promise<{ data: CapabilityManifestResponse }> => {
    const r = await client.get<unknown>(`/capability-versions/${versionId}/manifest`);
    return { data: parseCapabilityManifest(r.data) };
  },
  getByHash: async (hash: string): Promise<{ data: Manifest }> => {
    const r = await client.get<unknown>(`/manifests/${encodeURIComponent(hash)}`);
    assertManifest(r.data);
    return { data: r.data };
  },
  create: async (data: Manifest): Promise<{ data: { hash: string } }> => {
    const r = await client.post<unknown>('/manifests', data);
    const parsed = z.object({ hash: z.string().min(1) }).safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid manifest creation data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
};

/**
 * Client-side DAG validation. Mirrors server-side validation in
 * packages/shared/src/validation.ts. Reused by the editor for live feedback
 * before round-tripping to the server.
 */
export function validateDagClient(manifest: { nodes: Array<{ id: string }>; edges: Array<{ from: string; to: string }> }): string[] {
  const errors: string[] = [];
  const ids = new Set<string>();
  for (const n of manifest.nodes) {
    if (ids.has(n.id)) errors.push(`duplicate node id: ${n.id}`);
    ids.add(n.id);
  }
  for (const e of manifest.edges) {
    if (e.from === e.to) errors.push(`self-loop on ${e.from}`);
    if (!ids.has(e.from)) errors.push(`edge ${e.from}->${e.to} references missing source ${e.from}`);
    if (!ids.has(e.to)) errors.push(`edge ${e.from}->${e.to} references missing target ${e.to}`);
  }
  const adj = new Map<string, string[]>();
  for (const id of ids) adj.set(id, []);
  for (const e of manifest.edges) {
    if (ids.has(e.from) && ids.has(e.to)) adj.get(e.from)!.push(e.to);
  }
  const color = new Map<string, 0 | 1 | 2>();
  for (const id of ids) color.set(id, 0);
  const visit = (node: string, stack: string[]): void => {
    const c = color.get(node);
    if (c === 1) { errors.push(`cycle: ${[...stack, node].join(' -> ')}`); return; }
    if (c === 2) return;
    color.set(node, 1);
    stack.push(node);
    for (const next of adj.get(node) ?? []) visit(next, stack);
    stack.pop();
    color.set(node, 2);
  };
  for (const id of ids) if (color.get(id) === 0) visit(id, []);
  return errors;
}

export const webhookApi = {
  list: async (): Promise<{ data: { webhooks: WebhookSubscription[] } }> => {
    const r = await client.get<unknown>('/webhooks');
    const parsed = z.object({ webhooks: z.array(WebhookSubscriptionSchema) }).safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid webhook data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  create: async (data: { organizationId: string; label: string; url: string; events: string[] }): Promise<{ data: WebhookSubscription }> => {
    const r = await client.post<unknown>('/webhooks', data);
    const parsed = WebhookSubscriptionSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid webhook data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  update: async (id: string, data: { url?: string; events?: string[]; active?: boolean }): Promise<{ data: WebhookSubscription }> => {
    const r = await client.put<unknown>(`/webhooks/${encodeURIComponent(id)}`, data);
    const parsed = WebhookSubscriptionSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid webhook data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  delete: (id: string) => client.delete(`/webhooks/${id}`),
};

export const apiKeyApi = {
  list: async (): Promise<{ data: { keys: ApiKeySummary[] } }> => {
    const r = await client.get<unknown>('/api-keys');
    const parsed = z.object({ keys: z.array(ApiKeySummarySchema) }).safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid API-key data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  create: async (data: { name: string; role: ApiKeyRole; userId: string }): Promise<{ data: IssuedApiKey }> => {
    const r = await client.post<unknown>('/api-keys', data);
    const parsed = IssuedApiKeySchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned an invalid API key.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  revoke: (id: string) => client.delete(`/api-keys/${id}`),
};

export const userApi = {
  list: async (): Promise<{ data: { users: User[] } }> => {
    const r = await client.get<unknown>('/users');
    const parsed = z.object({ users: z.array(UserSchema) }).safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid user data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  updateRole: async (id: string, role: UserRole): Promise<{ data: User }> => {
    const r = await client.put<unknown>(`/users/${encodeURIComponent(id)}/role`, { role });
    const parsed = UserSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid user data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  me: async (): Promise<{ data: User }> => {
    const r = await client.get<unknown>('/users/me');
    const parsed = UserSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid current-user data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
};

export const featureFlagApi = {
  list: async (): Promise<{ data: { flags: FeatureFlag[] } }> => {
    const r = await client.get<unknown>('/feature-flags');
    const parsed = z.object({ flags: z.array(FeatureFlagSchema) }).safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid feature-flag data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  update: async (name: string, data: { value: unknown; enabled?: boolean }): Promise<{ data: FeatureFlag }> => {
    const r = await client.put<unknown>(`/feature-flags/${encodeURIComponent(name)}`, data);
    const parsed = FeatureFlagSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid feature-flag data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
};

// ---- Phase 5 surface: repositories, branches, contents, commits, MRs, signing, evals, vault, search, cost

export interface BranchItem {
  id: string;
  repositoryId: string;
  name: string;
  headCommitOid: string | null;
  isProtected: boolean;
  createdAt: string;
  updatedAt: string;
}
export interface TagItem {
  id: string;
  repositoryId: string;
  name: string;
  commitOid: string;
  message: string | null;
  taggerId: string;
  createdAt: string;
}
export interface RepoEntry {
  path: string;
  blobOid: string;
  size: number;
}

export interface CommitItem {
  oid: string;
  repositoryId: string;
  ref: string;
  treeOid: string;
  parents: string[];
  authorId: string;
  message: string;
  timestamp: string;
  signature?: string | null | undefined;
  signedKeyId?: string | null | undefined;
  signedAt?: string | null | undefined;
}

export interface RepositorySummary {
  id: string;
  workspaceId: string;
  name: string;
  slug: string;
  description: string | null;
  defaultBranch: string;
  visibility: 'private' | 'internal' | 'public';
  minApprovers: number;
  requireSignedReleases: boolean;
  updatedAt: string;
}

const RepositorySummarySchema = z.object({
  id: z.string().min(1),
  workspaceId: z.string().min(1),
  name: z.string().min(1),
  slug: z.string().min(1),
  description: z.string().nullable(),
  defaultBranch: z.string().min(1),
  visibility: z.enum(['private', 'internal', 'public']),
  minApprovers: z.number().int().nonnegative(),
  requireSignedReleases: z.boolean(),
  updatedAt: z.string(),
});

const BranchItemSchema = z.object({
  id: z.string(),
  repositoryId: z.string(),
  name: z.string(),
  headCommitOid: z.string().nullable(),
  isProtected: z.boolean(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

const RepoEntrySchema = z.object({
  path: z.string(),
  blobOid: z.string(),
  size: z.number().int().nonnegative(),
});

const CommitItemSchema = z.object({
  oid: z.string(),
  repositoryId: z.string(),
  ref: z.string(),
  treeOid: z.string(),
  parents: z.array(z.string()),
  authorId: z.string(),
  message: z.string(),
  timestamp: z.string(),
  signature: z.string().nullable().optional(),
  signedKeyId: z.string().nullable().optional(),
  signedAt: z.string().nullable().optional(),
});

function parseRepository(raw: unknown): RepositorySummary {
  const parsed = RepositorySummarySchema.safeParse(raw);
  if (!parsed.success) {
    throw new ApiError('The server returned invalid repository data.', { code: 'INVALID_RESPONSE' });
  }
  return parsed.data;
}

function parseRepositoryList(raw: unknown): RepositorySummary[] {
  const items = unwrapList<unknown>(raw);
  const parsed = z.array(RepositorySummarySchema).safeParse(items);
  if (!parsed.success) {
    throw new ApiError('The server returned invalid repository data.', { code: 'INVALID_RESPONSE' });
  }
  return parsed.data;
}

function parseBranchList(raw: unknown): BranchItem[] {
  const parsed = z.array(BranchItemSchema).safeParse(unwrapList<unknown>(raw));
  if (!parsed.success) throw new ApiError('The server returned invalid branch data.', { code: 'INVALID_RESPONSE' });
  return parsed.data;
}

function parseRepoEntryList(raw: unknown): RepoEntry[] {
  const parsed = z.array(RepoEntrySchema).safeParse(unwrapList<unknown>(raw));
  if (!parsed.success) throw new ApiError('The server returned invalid repository contents.', { code: 'INVALID_RESPONSE' });
  return parsed.data;
}

function parseCommitList(raw: unknown): CommitItem[] {
  const parsed = z.array(CommitItemSchema).safeParse(unwrapList<unknown>(raw));
  if (!parsed.success) throw new ApiError('The server returned invalid commit data.', { code: 'INVALID_RESPONSE' });
  return parsed.data;
}

export const repoApi = {
  list: (workspaceId: string): Promise<RepositorySummary[]> =>
    client.get<unknown>(`/repos?workspaceId=${encodeURIComponent(workspaceId)}`).then((r) => parseRepositoryList(r.data)),
  get: (id: string): Promise<RepositorySummary> => client.get<unknown>(`/repos/${id}`).then((r) => parseRepository(r.data)),
  create: (input: {
    workspaceId: string;
    name: string;
    slug?: string;
    description?: string;
    defaultBranch?: string;
    visibility?: 'private' | 'internal' | 'public';
    minApprovers?: number;
    requireSignedReleases?: boolean;
  }) => client.post('/repos', input).then((r) => r.data),
  listBranches: async (repoId: string): Promise<BranchItem[]> => {
    const r = await client.get<unknown>(`/repos/${repoId}/branches`);
    return parseBranchList(r.data);
  },
  listTags: (repoId: string) => client.get(`/repos/${repoId}/tags`).then((r) => r.data),
  listContents: async (repoId: string, ref = 'main'): Promise<RepoEntry[]> => {
    const r = await client.get<unknown>(`/repos/${repoId}/contents?ref=${encodeURIComponent(ref)}`);
    return parseRepoEntryList(r.data);
  },
  getFile: (repoId: string, path: string, ref = 'main'): Promise<{ data: { content?: string | undefined } }> =>
    client.get<unknown>(`/repos/${repoId}/contents/${encodeURIComponent(path)}?ref=${encodeURIComponent(ref)}`).then((r) => {
      const parsed = z.object({ content: z.string().optional() }).safeParse(r.data);
      if (!parsed.success) throw new ApiError('The server returned invalid file content.', { code: 'INVALID_RESPONSE' });
      return { data: parsed.data };
    }),
  putFile: (repoId: string, path: string, content: string, ref = 'main') =>
    client.put(`/repos/${repoId}/contents/${encodeURIComponent(path)}?ref=${encodeURIComponent(ref)}`, { path, content, ref }).then((r) => r.data),
  commit: (repoId: string, ref: string, message: string, parents?: string[]) =>
    client.post(`/repos/${repoId}/commits`, { ref, message, parents }).then((r) => r.data),
  listCommits: async (repoId: string, ref: string): Promise<CommitItem[]> => {
    const r = await client.get<unknown>(`/repos/${repoId}/commits?ref=${encodeURIComponent(ref)}`);
    return parseCommitList(r.data);
  },
  listMRs: async (repoId: string, status?: string): Promise<MergeRequest[]> => {
    const r = await client.get<unknown>(`/repos/${repoId}/merge-requests${status ? `?status=${status}` : ''}`);
    return parseMergeRequests(r.data);
  },
  openMR: (input: {
    repositoryId: string;
    title: string;
    description?: string;
    sourceBranch: string;
    targetBranch: string;
    sourceCommitOid: string;
  }) => client.post(`/repos/${input.repositoryId}/merge-requests`, input).then((r) => r.data),
  getMR: async (id: string): Promise<{
    mr: MergeRequest;
    approvals: MergeRequestApproval[];
    comments: MergeRequestComment[];
  }> => {
    const r = await client.get<unknown>(`/merge-requests/${id}`);
    return parseMergeRequestDetail(r.data);
  },
  decideMR: (id: string, decision: 'approve' | 'request_changes', comment?: string) =>
    client.post(`/merge-requests/${id}/decisions`, { decision, comment }).then((r) => r.data),
  commentMR: (id: string, body: string, path?: string) =>
    client.post(`/merge-requests/${id}/comments`, { body, path }).then((r) => r.data),
  mergeMR: (id: string, mergeCommitOid: string) =>
    client.post(`/merge-requests/${id}/merge`, { mergeCommitOid }).then((r) => r.data),
};

export const signingKeysApi = {
  list: async (organizationId: string): Promise<SigningKey[]> => {
    const r = await client.get<unknown>(`/orgs/${organizationId}/signing-keys`);
    const parsed = SigningKeyListSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid signing keys.', { code: 'INVALID_RESPONSE' });
    return parsed.data;
  },
  upload: async (organizationId: string, label: string, publicKeyPem: string): Promise<SigningKey> => {
    const r = await client.post<unknown>(`/orgs/${organizationId}/signing-keys`, { organizationId, label, publicKeyPem });
    const parsed = SigningKeySchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned an invalid signing key.', { code: 'INVALID_RESPONSE' });
    return parsed.data;
  },
  deactivate: async (organizationId: string, keyId: string): Promise<SigningKey> => {
    const r = await client.delete<unknown>(`/orgs/${organizationId}/signing-keys/${keyId}`);
    const parsed = SigningKeySchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned an invalid signing key.', { code: 'INVALID_RESPONSE' });
    return parsed.data;
  },
};

export const evalSuiteApi = {
  list: async (capabilityId?: string): Promise<EvalSuite[]> => {
    const r = await client.get<unknown>(`/eval-suites${capabilityId ? `?capabilityId=${capabilityId}` : ''}`);
    return parseEvalSuites(r.data);
  },
  get: async (id: string): Promise<{ data: { suite: EvalSuite; versions: EvalSuiteVersion[] } }> => {
    const r = await client.get<unknown>(`/eval-suites/${id}`);
    return { data: parseEvalSuiteDetail(r.data) };
  },
  runs: async (id: string): Promise<EvalSuiteRun[]> => {
    const r = await client.get<unknown>(`/eval-suites/${id}/runs`);
    return parseEvalSuiteRuns(r.data);
  },
  runDetail: async (suiteId: string, runId: string): Promise<{ run: EvalSuiteRun; results: unknown[] }> => {
    const r = await client.get<{ run: unknown; results: unknown[] }>(`/eval-suites/${suiteId}/runs/${runId}`);
    const run = EvalSuiteRunSchema.safeParse(r.data.run);
    if (!run.success || !Array.isArray(r.data.results)) throw new ApiError('The server returned invalid eval suite run details.', { code: 'INVALID_RESPONSE' });
    return { run: run.data, results: r.data.results };
  },
  create: (input: {
    capabilityId: string;
    name: string;
    description?: string;
    passThreshold?: number;
    borderlineBand?: number;
    initialGraders?: Array<{ name: string; kind: string; weight: number; config: unknown }>;
  }) => client.post('/eval-suites', input).then((r) => r.data),
  run: (suiteId: string, trials: unknown) =>
    client.post(`/eval-suites/${suiteId}/run`, trials).then((r) => r.data),
  gate: (repoId: string, trials: unknown) =>
    client.post(`/repos/${repoId}/eval-gate`, { trials }).then((r) => r.data),
};

export const mutationProposalApi = {
  list: async (filter: { status?: MutationProposal['status']; sourceHash?: string } = {}): Promise<MutationProposal[]> => {
    const params = new URLSearchParams();
    if (filter.status) params.set('status', filter.status);
    if (filter.sourceHash) params.set('sourceHash', filter.sourceHash);
    const suffix = params.toString() ? `?${params.toString()}` : '';
    const r = await client.get<unknown>(`/mutation-proposals${suffix}`);
    return parseMutationProposals(r.data);
  },
  validate: async (id: string): Promise<MutationProposal> => {
    const r = await client.post<unknown>(`/mutation-proposals/${id}/validate`);
    const parsed = MutationProposalSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid mutation proposal data.', { code: 'INVALID_RESPONSE' });
    return parsed.data;
  },
  attachEvaluation: async (id: string, input: { evaluationRunId: string; baselineScore: number }): Promise<MutationProposal> => {
    const r = await client.post<unknown>(`/mutation-proposals/${id}/evaluation`, input);
    const parsed = MutationProposalSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid mutation proposal data.', { code: 'INVALID_RESPONSE' });
    return parsed.data;
  },
  decide: async (id: string, decision: 'approve' | 'reject' | 'abandon', reason: string): Promise<MutationProposal> => {
    const r = await client.post<unknown>(`/mutation-proposals/${id}/decision`, { decision, reason });
    const parsed = MutationProposalSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid mutation proposal data.', { code: 'INVALID_RESPONSE' });
    return parsed.data;
  },
  promote: async (id: string, environment: ReleaseEnvironment): Promise<{ proposal: MutationProposal; release: Release }> => {
    const r = await client.post<unknown>(`/mutation-proposals/${id}/promote`, { environment });
    if (!r.data || typeof r.data !== 'object') throw new ApiError('The server returned invalid promotion data.', { code: 'INVALID_RESPONSE' });
    const value = r.data as { proposal?: unknown; release?: unknown };
    const proposal = MutationProposalSchema.safeParse(value.proposal);
    const release = ReleaseSchema.safeParse(value.release);
    if (!proposal.success || !release.success) throw new ApiError('The server returned invalid promotion data.', { code: 'INVALID_RESPONSE' });
    return { proposal: proposal.data, release: release.data };
  },
};

export const vaultApi = {
  listSecrets: async (organizationId: string): Promise<{ data: VaultSecretMetadata[] }> => {
    const r = await client.get<unknown>(`/vault/secrets?organizationId=${encodeURIComponent(organizationId)}`);
    const parsed = z.array(VaultSecretMetadataSchema).safeParse(unwrapList<unknown>(r.data));
    if (!parsed.success) throw new ApiError('The server returned invalid vault secret metadata.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  listKeys: async (): Promise<VaultKeyringEntry[]> => {
    const r = await client.get<unknown>('/vault/keys');
    return parseVaultKeyring(r.data);
  },
  rotateKey: async (label: string, reencrypt = true): Promise<{ data: VaultKeyRotation }> => {
    const r = await client.post<unknown>('/vault/keys/rotate', { label, reencrypt });
    const parsed = VaultKeyRotationSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid vault rotation data.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
  writeSecret: async (organizationId: string, name: string, value: string): Promise<{ data: VaultSecretMetadata }> => {
    const r = await client.post<unknown>('/vault/secrets', { organizationId, name, value });
    const parsed = VaultSecretMetadataSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid vault secret metadata.', { code: 'INVALID_RESPONSE' });
    return { data: parsed.data };
  },
};

export const retentionApi = {
  get: (organizationId: string) =>
    client.get(`/orgs/${organizationId}/retention`).then((r) => r.data),
  set: (organizationId: string, days: number) =>
    client.put(`/orgs/${organizationId}/retention`, { organizationId, days }).then((r) => r.data),
  sweep: (organizationId: string) =>
    client.post(`/orgs/${organizationId}/retention/sweep`).then((r) => r.data),
};

export const costApi = {
  forOrg: async (organizationId: string, days = 30): Promise<{ data: CostRollup[] }> => {
    const r = await client.get<unknown>(`/analytics/cost?organizationId=${encodeURIComponent(organizationId)}&days=${days}`);
    return { data: parseCostRollups(r.data) };
  },
  ingest: (row: { capabilityId: string; input?: number; output?: number; costMicros?: number; executions?: number }) =>
    client.post('/analytics/rollups', row),
};

export interface CostBudget {
  id: string;
  organizationId: string;
  label: string;
  period: 'weekly' | 'monthly';
  limitMicros: number;
  alertThreshold: number;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
  lastAlertedAt: string | null;
}

export interface UserQuota {
  id: string;
  organizationId: string;
  userId: string;
  label: string;
  dailyRuns: number | null;
  dailyTokens: number | null;
  dailyCostMicros: number | null;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
  usage: { runs: number; tokens: number; costMicros: number };
}

export interface CostForecast {
  snapshot: {
    id: string;
    organizationId: string;
    periodStart: string;
    periodEnd: string;
    spendMicros: number;
    projectedMicros: number;
    bandLowMicros: number;
    bandHighMicros: number;
    windowDays: number;
    computedAt: string;
  } | null;
  alerts: Array<{
    budgetId: string;
    label: string;
    projectedMicros: number;
    limitMicros: number;
    alertThreshold: number;
    fraction: number;
  }>;
}

const CostBudgetSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  label: z.string(),
  period: z.enum(['weekly', 'monthly']),
  limitMicros: z.number().int().nonnegative(),
  alertThreshold: z.number().min(0).max(1),
  enabled: z.boolean(),
  createdAt: z.string(),
  updatedAt: z.string(),
  lastAlertedAt: z.string().nullable(),
});

const UserQuotaSchema = z.object({
  id: z.string(),
  organizationId: z.string(),
  userId: z.string(),
  label: z.string(),
  dailyRuns: z.number().int().nullable(),
  dailyTokens: z.number().int().nullable(),
  dailyCostMicros: z.number().int().nullable(),
  enabled: z.boolean(),
  createdAt: z.string(),
  updatedAt: z.string(),
  usage: z.object({ runs: z.number().int(), tokens: z.number().int(), costMicros: z.number().int() }),
});

const CostForecastSchema = z.object({
  snapshot: z.object({
    id: z.string(),
    organizationId: z.string(),
    periodStart: z.string(),
    periodEnd: z.string(),
    spendMicros: z.number(),
    projectedMicros: z.number(),
    bandLowMicros: z.number(),
    bandHighMicros: z.number(),
    windowDays: z.number().int(),
    computedAt: z.string(),
  }).nullable(),
  alerts: z.array(z.object({
    budgetId: z.string(),
    label: z.string(),
    projectedMicros: z.number(),
    limitMicros: z.number(),
    alertThreshold: z.number(),
    fraction: z.number(),
  })),
});

export const budgetApi = {
  list: async (organizationId: string): Promise<{ items: CostBudget[] }> => {
    const response = await client.get<unknown>('/admin/budgets', { params: { organizationId } });
    const parsed = z.object({ items: z.array(CostBudgetSchema) }).safeParse(response.data);
    if (!parsed.success) throw new ApiError('The server returned invalid budget data.', { code: 'INVALID_RESPONSE' });
    return parsed.data;
  },
  create: async (input: {
    organizationId: string;
    label: string;
    period: CostBudget['period'];
    limitMicros: number;
    alertThreshold: number;
    enabled: boolean;
  }): Promise<CostBudget> => {
    const response = await client.post<unknown>('/admin/budgets', input);
    return CostBudgetSchema.parse(response.data);
  },
  update: async (id: string, fields: Partial<Pick<CostBudget, 'label' | 'period' | 'limitMicros' | 'alertThreshold' | 'enabled'>>): Promise<CostBudget> => {
    const response = await client.patch<unknown>(`/admin/budgets/${encodeURIComponent(id)}`, fields);
    return CostBudgetSchema.parse(response.data);
  },
  remove: (id: string): Promise<void> => client.delete(`/admin/budgets/${encodeURIComponent(id)}`).then(() => undefined),
  forecast: async (organizationId: string, windowDays = 30): Promise<CostForecast> => {
    const response = await client.get<unknown>('/admin/cost-forecast', { params: { organizationId, windowDays } });
    return CostForecastSchema.parse(response.data);
  },
};

export const userQuotaApi = {
  list: async (organizationId: string): Promise<{ items: UserQuota[] }> => {
    const response = await client.get<unknown>('/admin/user-quotas', { params: { organizationId } });
    const parsed = z.object({ items: z.array(UserQuotaSchema) }).safeParse(response.data);
    if (!parsed.success) throw new ApiError('The server returned invalid user quota data.', { code: 'INVALID_RESPONSE' });
    return parsed.data;
  },
  create: async (input: {
    organizationId: string;
    userId: string;
    label: string;
    dailyRuns: number | null;
    dailyTokens: number | null;
    dailyCostMicros: number | null;
    enabled: boolean;
  }): Promise<UserQuota> => UserQuotaSchema.parse((await client.post('/admin/user-quotas', input)).data),
  update: async (id: string, fields: Partial<Pick<UserQuota, 'label' | 'dailyRuns' | 'dailyTokens' | 'dailyCostMicros' | 'enabled'>>): Promise<UserQuota> => UserQuotaSchema.parse((await client.patch(`/admin/user-quotas/${encodeURIComponent(id)}`, fields)).data),
  remove: (id: string): Promise<void> => client.delete(`/admin/user-quotas/${encodeURIComponent(id)}`).then(() => undefined),
};

export interface TraceRunSummary {
  id: string;
  organizationId: string;
  executionId: string | null;
  environment: string;
  name: string;
  startTime: string;
  endTime: string | null;
  status: 'running' | 'success' | 'error';
  totalTokens: number;
  totalCostUsd: number;
  model: string | null;
}

export interface TraceOperationalSummary {
  runs: number;
  errors: number;
  averageLatencyMs: number;
  tokens: number;
  cost: number;
  models: Array<{ model: string; runs: number; errors: number; tokens: number; cost: number }>;
}

export interface TracePromptRisk {
  promptKey: string;
  runs: number;
  errors: number;
  errorRate: number;
  tokens: number;
  cost: number;
  actors: number;
  lastSeen: string;
  signals: Array<'error-rate' | 'token-burn' | 'volume'>;
  risk: 'medium' | 'high';
}

export interface TraceSpan {
  id: string;
  traceRunId: string;
  parentSpanId: string | null;
  name: string;
  kind: 'internal' | 'llm' | 'tool' | 'retrieval' | 'agent';
  startTime: string;
  endTime: string | null;
  status: 'ok' | 'error';
  attributes: Record<string, unknown>;
  model: string | null;
  promptTokens: number | null;
  completionTokens: number | null;
  totalTokens: number | null;
  costUsd: number | null;
  inputText: string | null;
  outputText: string | null;
}

export interface EvidenceRecord {
  id: string;
  eventType: string;
  schemaVersion: string;
  occurredAt: string;
  organizationId: string;
  correlationId: string;
  traceId: string | null;
  executionId: string | null;
  agentHash: string | null;
  stepId: string | null;
  retentionClass: string;
  payload: unknown;
  payloadHash: string;
  createdAt: string;
}

const EvidenceRecordSchema = z.object({
  id: z.string(),
  eventType: z.string(),
  schemaVersion: z.string(),
  occurredAt: z.string(),
  organizationId: z.string(),
  correlationId: z.string(),
  traceId: z.string().nullable(),
  executionId: z.string().nullable(),
  agentHash: z.string().nullable(),
  stepId: z.string().nullable(),
  retentionClass: z.string(),
  payload: z.unknown(),
  payloadHash: z.string(),
  createdAt: z.string(),
}) satisfies z.ZodType<EvidenceRecord>;

const EvidencePageSchema = z.object({
  items: z.array(EvidenceRecordSchema),
  total: z.number().int().nonnegative(),
});

export interface PlaygroundRun {
  content: string;
  provider: string;
  model: string;
  promptTokens: number;
  completionTokens: number;
  costUsd: number;
  cacheHit: boolean;
  latencyMs: number;
}

export const playgroundApi = {
  complete: (data: {
    prompt: string;
    model: string;
    provider: 'openai' | 'anthropic' | 'bedrock' | 'custom' | 'simulated';
    temperature?: number;
    baseUrl?: string;
    apiKey?: string;
  }) => client.post<PlaygroundRun>('/playground/complete', data).then((r) => r.data),
  sweep: (data: {
    base: {
      prompt: string;
      model: string;
    provider: 'openai' | 'anthropic' | 'bedrock' | 'custom' | 'simulated';
      baseUrl?: string;
      apiKey?: string;
    };
    variants: Array<{ prompt: string; temperature: number }>;
  }) =>
    client
      .post<{
        base: { model: string; provider: string };
        variants: Array<{
          variant: { prompt: string; temperature: number };
          status: 'fulfilled' | 'rejected';
          value?: PlaygroundRun;
          error?: string;
        }>;
      }>('/playground/sweep', data)
      .then((r) => r.data),
};

export const traceApi = {
  list: (opts: { page?: number; pageSize?: number; environment?: string; status?: string; nameLike?: string } = {}) =>
    client
      .get<{ items: TraceRunSummary[]; total: number }>('/traces', { params: opts })
      .then((r) => r.data),
  get: (id: string) =>
    client.get<{ run: TraceRunSummary; spans: TraceSpan[] }>(`/traces/${id}`).then((r) => r.data),
  evidence: (id: string, workspaceId?: string) =>
    client.get<{ traceId: string; items: EvidenceRecord[]; total: number }>(`/traces/${id}/evidence`, { params: workspaceId ? { workspaceId } : undefined }).then((r) => r.data),
  rollup: (days = 30) =>
    client
      .get<{ days: number; items: Array<{ day: string; tokens: number; cost: number; runs: number }> }>(
        `/traces/rollup`,
        { params: { days } },
      )
      .then((r) => r.data),
  summary: (days = 7) =>
    client.get<{ orgId: string; days: number; summary: TraceOperationalSummary }>('/traces/summary', { params: { days } }).then((r) => r.data),
  promptRisk: (days = 30, limit = 25) =>
    client.get<{ orgId: string; days: number; limit: number; items: TracePromptRisk[] }>('/traces/prompt-risk', { params: { days, limit } }).then((r) => r.data),
};

export const evidenceApi = {
  list: async (options: { limit?: number; before?: string; eventType?: string; agentHash?: string; workspaceId?: string } = {}): Promise<{ data: z.infer<typeof EvidencePageSchema> }> => {
    const r = await client.get<unknown>('/evidence', { params: options });
    return { data: EvidencePageSchema.parse(r.data) };
  },
  export: async (options: { limit?: number; before?: string; eventType?: string; workspaceId?: string } = {}): Promise<{
    schemaVersion: string;
    organizationId: string;
    exportedAt: string;
    items: EvidenceRecord[];
  }> => {
    const r = await client.get<unknown>('/evidence/export', { params: options });
    return z.object({
      schemaVersion: z.string(),
      organizationId: z.string(),
      exportedAt: z.string(),
      items: z.array(EvidenceRecordSchema),
    }).parse(r.data);
  },
};

export interface TraceScore {
  id: string;
  evaluator: string;
  name: string;
  value: number | null;
  label: string | null;
  rationale: string | null;
  createdAt: string;
}

export interface UserDailyUsage {
  day: string;
  runs: number;
  tokens: number;
  cost: number;
}

export interface UserRollup {
  actorId: string;
  runs: number;
  tokens: number;
  cost: number;
  days: number;
}

export const analyticsApi = {
  userPerDay: (userId: string, days = 30) =>
    client
      .get<{ userId: string; days: number; perDay: UserDailyUsage[] }>(
        `/analytics/users/${encodeURIComponent(userId)}`,
        { params: { days } },
      )
      .then((r) => r.data),
  leaderboard: (days = 30, limit = 25) =>
    client
      .get<{
        orgId: string;
        days: number;
        limit: number;
        items: UserRollup[];
      }>('/analytics/leaderboard', { params: { days, limit } })
      .then((r) => r.data),
  orgTotals: (days = 30) =>
    client
      .get<{
        orgId: string;
        days: number;
        totals: { runs: number; tokens: number; cost: number; activeDays: number };
      }>('/analytics/org-totals', { params: { days } })
      .then((r) => r.data),
};

export interface AuditReportEntry {
  id: string;
  timestamp: string;
  actor: string;
  action: string;
  resource: string;
  details: string;
}

export interface AuditReport {
  id: string;
  generatedAt: string;
  generatedBy: string | null;
  organizationId: string;
  range: { from: string | null; to: string | null };
  filters: Record<string, string | number | undefined>;
  entryCount: number;
  chainValid: boolean;
  chainHead: string;
  chainVerifiedAt: string;
  signature: { algorithm: string; value: string };
  entries: AuditReportEntry[];
}

export const auditApi = {
  list: async (params?: { resource?: string; action?: string }): Promise<{ data: AuditEntry[] }> => {
    const r = await client.get<unknown>('/audit', { params });
    return { data: parseAuditEntries(r.data) };
  },
  report: (opts: {
    fromTime?: string;
    toTime?: string;
    actor?: string;
    resource?: string;
    action?: string;
    limit?: number;
  } = {}) => {
    const params: Record<string, string | number> = {};
    if (opts.fromTime) params['fromTime'] = opts.fromTime;
    if (opts.toTime) params['toTime'] = opts.toTime;
    if (opts.actor) params['actor'] = opts.actor;
    if (opts.resource) params['resource'] = opts.resource;
    if (opts.action) params['action'] = opts.action;
    if (opts.limit) params['limit'] = opts.limit;
    return client
      .get<ArrayBuffer>('/audit/report', {
        params,
        responseType: 'arraybuffer',
      })
      .then((r) => {
        const text = new TextDecoder().decode(r.data);
        return JSON.parse(text) as AuditReport;
      });
  },
};

export interface TeamSummary {
  id: string;
  organizationId: string;
  name: string;
  slug: string;
  description: string;
  createdAt: string;
  updatedAt: string;
}

export interface TeamMember {
  teamId: string;
  userId: string;
  role: 'owner' | 'admin' | 'member' | 'viewer';
  createdAt: string;
}

export interface SsoConfigView {
  configured: boolean;
  provider: string | undefined;
  issuer: string | undefined;
  clientId: string | undefined;
  scopes: string | undefined;
  audience: string | null | undefined;
  groupsClaim: string | undefined;
  emailClaim: string | undefined;
  nameClaim: string | undefined;
  enabled: boolean | undefined;
}

const TeamSummarySchema = z.object({
  id: z.string().min(1),
  organizationId: z.string().min(1),
  name: z.string().min(1),
  slug: z.string().min(1),
  description: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
});

const TeamMemberSchema = z.object({
  teamId: z.string().min(1),
  userId: z.string().min(1),
  role: z.enum(['owner', 'admin', 'member', 'viewer']),
  createdAt: z.string(),
});

const TeamListSchema = z.object({ items: z.array(TeamSummarySchema) });
const SsoConfigSchema = z.object({
  configured: z.boolean(),
  provider: z.string().optional(),
  issuer: z.string().optional(),
  clientId: z.string().optional(),
  scopes: z.string().optional(),
  audience: z.string().nullable().optional(),
  groupsClaim: z.string().optional(),
  emailClaim: z.string().optional(),
  nameClaim: z.string().optional(),
  enabled: z.boolean().optional(),
});
const SsoUpdateSchema = z.object({ status: z.literal('ok'), provider: z.string().min(1) });

export const teamApi = {
  list: async (): Promise<{ items: TeamSummary[] }> => {
    const r = await client.get<unknown>('/teams');
    const parsed = TeamListSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid teams.', { code: 'INVALID_RESPONSE' });
    return parsed.data;
  },
  create: async (data: { name: string; slug: string; description?: string }): Promise<TeamSummary> => {
    const r = await client.post<unknown>('/teams', data);
    const parsed = TeamSummarySchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned an invalid team.', { code: 'INVALID_RESPONSE' });
    return parsed.data;
  },
  addMember: async (teamId: string, data: { userId: string; role?: TeamMember['role'] }): Promise<TeamMember> => {
    const r = await client.post<unknown>(`/teams/${teamId}/members`, data);
    const parsed = TeamMemberSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned an invalid team member.', { code: 'INVALID_RESPONSE' });
    return parsed.data;
  },
  removeMember: async (teamId: string, userId: string): Promise<void> => {
    await client.delete(`/teams/${teamId}/members/${userId}`);
  },
  ssoGet: async (): Promise<SsoConfigView> => {
    const r = await client.get<unknown>('/auth/oidc/config');
    const parsed = SsoConfigSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned invalid SSO configuration.', { code: 'INVALID_RESPONSE' });
    return {
      configured: parsed.data.configured,
      provider: parsed.data.provider,
      issuer: parsed.data.issuer,
      clientId: parsed.data.clientId,
      scopes: parsed.data.scopes,
      audience: parsed.data.audience,
      groupsClaim: parsed.data.groupsClaim,
      emailClaim: parsed.data.emailClaim,
      nameClaim: parsed.data.nameClaim,
      enabled: parsed.data.enabled,
    };
  },
  ssoSet: async (data: {
    provider: string;
    issuer: string;
    clientId: string;
    clientSecret: string;
    scopes?: string;
    audience?: string;
    groupsClaim?: string;
    emailClaim?: string;
    nameClaim?: string;
  }): Promise<{ status: 'ok'; provider: string }> => {
    const r = await client.post<unknown>('/auth/oidc/config', data);
    const parsed = SsoUpdateSchema.safeParse(r.data);
    if (!parsed.success) throw new ApiError('The server returned an invalid SSO update response.', { code: 'INVALID_RESPONSE' });
    return parsed.data;
  },
};

export const traceScoreApi = {
  list: (traceRunId: string) =>
    client
      .get<{ run: TraceRunSummary; items: TraceScore[]; total: number }>(`/traces/${traceRunId}/scores`)
      .then((r) => r.data),
  autoEval: (traceRunId: string, opts: { judgeModel?: string; judgePrompt?: string } = {}) =>
    client
      .post<{ traceRunId: string; written: number }>(`/traces/${traceRunId}/auto-eval`, opts)
      .then((r) => r.data),
  summary: (days = 7, evaluator?: string) =>
    client
      .get<{ orgId: string; days: number; totals: number; perEvaluator: Array<{ evaluator: string; count: number }> }>(
        `/scores/summary`,
        { params: { days, ...(evaluator ? { evaluator } : {}) } },
      )
      .then((r) => r.data),
};

export const searchApi = {
  q: async (q: string, type?: string): Promise<SearchResult[]> => {
    const r = await client.get<unknown>(`/search?q=${encodeURIComponent(q)}${type ? `&type=${encodeURIComponent(type)}` : ''}`);
    return parseSearchResults(r.data);
  },
};
