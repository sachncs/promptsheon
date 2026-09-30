import type {
  Repository,
  RepositoryCreateInput,
  EvalSuite,
  EvalSuiteVersion,
  EvalSuiteRunInput,
  SigningKey,
  AgentSpecification,
} from '@promptsheon/shared';

export interface AgentSpecificationRecord {
  hash: string;
  workspaceId: string;
  schemaVersion: string;
  parentHash: string | null;
  author: string;
  changeReason: string;
  status: 'draft' | 'candidate' | 'published' | 'retired';
  createdAt: string;
  publishedAt: string | null;
  specification: AgentSpecification;
}

export type AgentSpecificationStatus = AgentSpecificationRecord['status'];

export type AgentSpecificationMetadata = Omit<AgentSpecificationRecord, 'specification'>;

export interface AgentSpecificationDiffEntry {
  path: string;
  before: unknown;
  after: unknown;
}

export type ExecutionJobState = 'queued' | 'running' | 'completed' | 'failed' | 'cancelled' | 'timed-out' | 'partially-completed';

export interface ExecutionJob {
  id: string;
  organizationId: string;
  workspaceId: string;
  agentHash: string;
  inputHash: string;
  idempotencyKey: string;
  state: ExecutionJobState;
  attempts: number;
  maxAttempts: number;
  resultJson: string | null;
  error: string | null;
  createdAt: string;
  startedAt: string | null;
  completedAt: string | null;
}

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

/** Evidence event types emitted by execution and policy subsystems. */
export type EvidenceEventType =
  | 'execution.started'
  | 'execution.completed'
  | 'execution.failed'
  | 'execution.cancelled'
  | 'model.called'
  | 'tool.called'
  | 'guardrail.decided'
  | 'permission.decided'
  | 'resource.consumed'
  | 'error.observed';

/** Redacted, append-only evidence captured for an execution. */
export interface EvidenceRecord {
  id: string;
  eventType: EvidenceEventType;
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

/**
 * Typed fetch wrapper over the public REST API.
 *
 * Auth is via an organization-scoped API key sent as a Bearer token.
 */
export interface SdkOptions {
  baseUrl?: string;
  apiKey?: string;
}

interface RequestOptions {
  method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  path: string;
  body?: unknown;
}

export class PromptsheonClient {
  private readonly baseUrl: string;
  private readonly apiKey: string | undefined;

  constructor(opts: SdkOptions = {}) {
    this.baseUrl = opts.baseUrl ?? 'http://127.0.0.1:8080';
    this.apiKey = opts.apiKey;
  }

  private async call<T>({ method, path, body }: RequestOptions): Promise<T> {
    const headers: Record<string, string> = { 'content-type': 'application/json' };
    if (this.apiKey) headers['authorization'] = `Bearer ${this.apiKey}`;
    const init: RequestInit = { method, headers };
    if (body !== undefined) init.body = JSON.stringify(body);
    const res = await fetch(`${this.baseUrl}/api${path}`, init);
    if (!res.ok) {
      const text = await res.text().catch(() => '');
      throw new Error(`${method} ${path} -> ${res.status}: ${text}`);
    }
    return (await res.json()) as T;
  }

  // ---- Repositories
  listRepos(workspaceId: string): Promise<Repository[]> {
    return this.call({ method: 'GET', path: `/repos?workspaceId=${workspaceId}` });
  }

  createRepo(input: RepositoryCreateInput): Promise<Repository> {
    return this.call({ method: 'POST', path: '/repos', body: input });
  }

  // ---- Branches / contents / commits
  listBranches(repoId: string): Promise<unknown[]> {
    return this.call({ method: 'GET', path: `/repos/${repoId}/branches` });
  }

  putFile(repoId: string, path: string, content: string, ref = 'main'): Promise<unknown> {
    return this.call({
      method: 'PUT',
      path: `/repos/${repoId}/contents/${path}?ref=${ref}`,
      body: { path, content, ref },
    });
  }

  commit(repoId: string, ref: string, message: string, parents?: string[]): Promise<unknown> {
    return this.call({
      method: 'POST',
      path: `/repos/${repoId}/commits`,
      body: { ref, message, parents },
    });
  }

  // ---- Merge requests
  openMergeRequest(input: {
    repositoryId: string;
    title: string;
    description?: string;
    sourceBranch: string;
    targetBranch: string;
    sourceCommitOid: string;
  }): Promise<unknown> {
    return this.call({ method: 'POST', path: `/repos/${input.repositoryId}/merge-requests`, body: input });
  }

  decideMergeRequest(id: string, decision: 'approve' | 'request_changes', comment?: string): Promise<unknown> {
    return this.call({ method: 'POST', path: `/merge-requests/${id}/decisions`, body: { decision, comment } });
  }

  // ---- Signing
  uploadSigningKey(organizationId: string, label: string, publicKeyPem: string): Promise<SigningKey> {
    return this.call({
      method: 'POST',
      path: `/orgs/${organizationId}/signing-keys`,
      body: { organizationId, label, publicKeyPem },
    });
  }

  signCommit(oid: string, keyId: string, signature: string): Promise<unknown> {
    return this.call({
      method: 'POST',
      path: `/commits/${oid}/sign`,
      body: { keyId, signature },
    });
  }

  verifyCommit(oid: string): Promise<{ valid: boolean; reason?: string | null }> {
    return this.call({ method: 'GET', path: `/commits/${oid}/verify` });
  }

  // ---- Eval suites
  listSuites(capabilityId?: string): Promise<EvalSuite[]> {
    const q = capabilityId ? `?capabilityId=${capabilityId}` : '';
    return this.call({ method: 'GET', path: `/eval-suites${q}` });
  }

  createSuite(input: {
    capabilityId: string;
    name: string;
    description?: string;
    passThreshold?: number;
    borderlineBand?: number;
    initialGraders?: Array<{ name: string; kind: string; weight: number; config: unknown }>;
  }): Promise<{ suite: EvalSuite; version: EvalSuiteVersion }> {
    return this.call({ method: 'POST', path: '/eval-suites', body: input });
  }

  runSuite(suiteId: string, input: Partial<EvalSuiteRunInput> & {
    trials?: Array<{
      caseId: string; output: string; transcript?: string; finalState?: Record<string, unknown>;
      toolCalls?: Array<{ tool: string; args: Record<string, unknown>; result?: unknown }>;
      referenceTranscript?: string;
    }>;
  }): Promise<unknown> {
    return this.call({ method: 'POST', path: `/eval-suites/${suiteId}/run`, body: input ?? {} });
  }

  evalGate(repoId: string, trials: Array<{ caseId: string; output: string; finalState?: Record<string, unknown> }>): Promise<unknown> {
    return this.call({ method: 'POST', path: `/repos/${repoId}/eval-gate`, body: { trials } });
  }

  validateAgentSpecification(workspaceId: string, specification: unknown): Promise<{ valid: boolean; specification?: AgentSpecification; issues?: unknown[] }> {
    return this.call({ method: 'POST', path: `/workspaces/${encodeURIComponent(workspaceId)}/agent-specifications/validate`, body: { specification } });
  }

  createAgentSpecification(input: { workspaceId: string; specification: AgentSpecification; changeReason: string; parentHash?: string | null }): Promise<AgentSpecificationRecord> {
    return this.call({
      method: 'POST',
      path: `/workspaces/${encodeURIComponent(input.workspaceId)}/agent-specifications`,
      body: { specification: input.specification, changeReason: input.changeReason, ...(input.parentHash === undefined ? {} : { parentHash: input.parentHash }) },
    });
  }

  listAgentSpecifications(workspaceId: string, options: { page?: number; pageSize?: number; status?: AgentSpecificationStatus } = {}): Promise<{ items: AgentSpecificationMetadata[]; total: number }> {
    const params = new URLSearchParams();
    if (options.page !== undefined) params.set('page', String(options.page));
    if (options.pageSize !== undefined) params.set('pageSize', String(options.pageSize));
    if (options.status !== undefined) params.set('status', options.status);
    const query = params.toString();
    return this.call({ method: 'GET', path: `/workspaces/${encodeURIComponent(workspaceId)}/agent-specifications${query ? `?${query}` : ''}` });
  }

  getAgentSpecification(workspaceId: string, hash: string): Promise<AgentSpecificationRecord> {
    return this.call({ method: 'GET', path: `/workspaces/${encodeURIComponent(workspaceId)}/agent-specifications/${encodeURIComponent(hash)}` });
  }

  diffAgentSpecifications(workspaceId: string, leftHash: string, rightHash: string): Promise<{ changes: AgentSpecificationDiffEntry[] }> {
    return this.call({ method: 'POST', path: `/workspaces/${encodeURIComponent(workspaceId)}/agent-specifications/diff`, body: { leftHash, rightHash } });
  }

  listAgentSpecificationLineage(workspaceId: string, hash: string): Promise<{ items: AgentSpecificationMetadata[] }> {
    return this.call({ method: 'GET', path: `/workspaces/${encodeURIComponent(workspaceId)}/agent-specifications/${encodeURIComponent(hash)}/lineage` });
  }

  publishAgentSpecification(workspaceId: string, hash: string): Promise<AgentSpecificationRecord> {
    return this.call({ method: 'POST', path: `/workspaces/${encodeURIComponent(workspaceId)}/agent-specifications/${encodeURIComponent(hash)}/publish` });
  }

  enqueueExecution(input: { workspaceId: string; agentHash: string; inputs: Record<string, unknown>; idempotencyKey: string; maxAttempts?: number }): Promise<ExecutionJob> {
    return this.call({ method: 'POST', path: `/workspaces/${encodeURIComponent(input.workspaceId)}/execution-jobs`, body: { agentHash: input.agentHash, inputs: input.inputs, idempotencyKey: input.idempotencyKey, ...(input.maxAttempts === undefined ? {} : { maxAttempts: input.maxAttempts }) } });
  }

  getExecution(workspaceId: string, id: string): Promise<ExecutionJob> {
    return this.call({ method: 'GET', path: `/workspaces/${encodeURIComponent(workspaceId)}/execution-jobs/${encodeURIComponent(id)}` });
  }

  cancelExecution(workspaceId: string, id: string): Promise<ExecutionJob> {
    return this.call({ method: 'POST', path: `/workspaces/${encodeURIComponent(workspaceId)}/execution-jobs/${encodeURIComponent(id)}/cancel` });
  }

  getExecutionMetrics(): Promise<ExecutionQueueMetrics> {
    return this.call({ method: 'GET', path: '/execution-jobs/metrics' });
  }

  /** List the newest evidence records visible to the authenticated organization. */
  listEvidence(options: { limit?: number; before?: string; eventType?: EvidenceEventType; agentHash?: string; traceId?: string } = {}): Promise<{ items: EvidenceRecord[]; total: number }> {
    const params = new URLSearchParams();
    if (options.limit !== undefined) params.set('limit', String(options.limit));
    if (options.before !== undefined) params.set('before', options.before);
    if (options.eventType !== undefined) params.set('eventType', options.eventType);
    if (options.agentHash !== undefined) params.set('agentHash', options.agentHash);
    if (options.traceId !== undefined) params.set('traceId', options.traceId);
    const query = params.toString();
    return this.call({ method: 'GET', path: `/evidence${query ? `?${query}` : ''}` });
  }

  /** List evidence for one trace in chronological order. */
  listTraceEvidence(traceId: string): Promise<{ traceId: string; items: EvidenceRecord[]; total: number }> {
    return this.call({ method: 'GET', path: `/traces/${encodeURIComponent(traceId)}/evidence` });
  }
}

// Re-export the framework integrations from sub-paths so callers
// can import everything from the package root.
export {
  withPromptsheon,
  type PromptsheonVercelOptions,
  type VercelLanguageModel,
} from './integrations/vercel-ai-sdk.js';
export {
  PromptsheonLLM,
  type PromptsheonLlamaindexOptions,
  type LlamaindexMessageLike,
  type LlamaindexCompletionRequest,
  type LlamaindexCompletionResponse,
} from './integrations/llamaindex.js';
export {
  PromptsheonGenerator,
  type PromptsheonHaystackOptions,
  type HaystackPrompt,
  type HaystackAnswer,
} from './integrations/haystack.js';
