import axios from 'axios';
import { z } from 'zod';
import { clearSession, getSession } from './session';

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
  const session = getSession();
  if (session?.apiKey) {
    config.headers.set('Authorization', `Bearer ${session.apiKey}`);
  }
  if (session?.userId) {
    config.headers.set('X-User-Id', session.userId);
  }
  if (session?.orgId) {
    config.headers.set('X-Org-Id', session.orgId);
  }
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

export interface WorkspaceRow {
  id: string;
  name: string;
  organization: string;
  createdAt: string;
  updatedAt: string;
}

export interface VaultKeyringEntry {
  id: number;
  label: string;
  fingerprint: string;
  active: boolean;
  createdAt: string;
  rotatedAt: string | null;
}

const VaultKeyringEntrySchema = z.object({
  id: z.number().int(),
  label: z.string(),
  fingerprint: z.string(),
  active: z.boolean(),
  createdAt: z.string(),
  rotatedAt: z.string().nullable(),
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
}

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

function parseEvalRun(raw: unknown): EvalRun {
  const parsed = EvalRunSchema.safeParse(raw);
  if (!parsed.success) {
    throw new ApiError('The server returned invalid eval run data.', { code: 'INVALID_RESPONSE' });
  }
  return parsed.data;
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

function parseCapabilities(raw: unknown): Capability[] {
  const parsed = z.array(CapabilitySchema).safeParse(unwrapList<unknown>(raw));
  if (!parsed.success) throw new ApiError('The server returned invalid capability data.', { code: 'INVALID_RESPONSE' });
  return parsed.data;
}

function parseReleases(raw: unknown): Release[] {
  const parsed = z.array(ReleaseSchema).safeParse(unwrapList<unknown>(raw));
  if (!parsed.success) throw new ApiError('The server returned invalid release data.', { code: 'INVALID_RESPONSE' });
  return parsed.data;
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

export const projectApi = {
  list: (workspaceId: string) => client.get('/projects', { params: { workspaceId } }),
  get: (id: string) => client.get(`/projects/${id}`),
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
  create: (data: { projectId: string; name: string; description?: string }) => client.post('/capabilities', data),
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
  transition: (id: string, to: 'draft' | 'review' | 'approved' | 'canary' | 'active' | 'rolled_back', reason?: string) =>
    client.post(`/releases/${id}/transition`, { to, ...(reason ? { reason } : {}) }),
  canary: (id: string, percent: number) => client.put(`/releases/${id}/canary`, { percent }),
  rollback: (id: string, toReleaseId?: string) => {
    const body: { toReleaseId?: string } = {};
    if (toReleaseId !== undefined) body.toReleaseId = toReleaseId;
    return client.post(`/releases/${id}/rollback`, body);
  },
};

export const executionApi = {
  list: (capabilityVersionId: string) => client.get('/executions', { params: { capabilityVersionId } }),
  get: (id: string) => client.get(`/executions/${id}`),
  execute: (data: { manifestHash: string; inputs: Record<string, unknown>; environment?: string; traceId?: string }) =>
    client.post('/executions', data),
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
    const session = getSession();
    const headers: Record<string, string> = {
      'content-type': 'application/json',
      accept: 'text/event-stream',
    };
    if (session?.apiKey) headers.Authorization = `Bearer ${session.apiKey}`;
    if (session?.userId) headers['X-User-Id'] = session.userId;
    if (session?.orgId) headers['X-Org-Id'] = session.orgId;
    void fetch(url, {
      method: 'POST',
      headers,
      body: JSON.stringify(data),
      signal: controller.signal,
    }).then(async (res) => {
      if (!res.ok || !res.body) return;
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';
      // eslint-disable-next-line no-constant-condition
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
  list: (capabilityId: string) => client.get('/datasets', { params: { capabilityId } }),
  get: (id: string) => client.get(`/datasets/${id}`),
  create: (data: { capabilityId: string; name: string; description?: string }) => client.post('/datasets', data),
  delete: (id: string) => client.delete(`/datasets/${id}`),
  getCases: (id: string) => client.get(`/datasets/${id}/cases`),
  addCase: (id: string, data: { inputs: string; expected: string; description?: string }) => client.post(`/datasets/${id}/cases`, data),
};

export const evalApi = {
  list: (releaseId?: string) => client.get('/eval-runs', { params: { releaseId } }),
  get: async (id: string): Promise<{ data: EvalRun }> => {
    const r = await client.get<unknown>(`/eval-runs/${id}`);
    return { data: parseEvalRun(r.data) };
  },
  create: (data: { releaseId: string; datasetId: string; scorer: string }) => client.post('/eval-runs', data),
  getResults: async (id: string): Promise<{ data: EvalResult[] }> => {
    const r = await client.get<unknown>(`/eval-runs/${id}/results`);
    return { data: parseEvalResults(r.data) };
  },
};

export const alertApi = {
  listRules: () => client.get('/alert-rules'),
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
  list: () => client.get('/schedules'),
  get: (id: string) => client.get(`/schedules/${id}`),
  create: (data: { workspaceId: string; releaseId: string; kind: string; cron: string }) => client.post('/schedules', data),
  delete: (id: string) => client.delete(`/schedules/${id}`),
};

export const settingsApi = {
  list: () => client.get('/settings'),
  get: (key: string) => client.get(`/settings/${key}`),
  set: (key: string, value: unknown) => client.put(`/settings/${key}`, { value }),
};

export const preconditionApi = {
  list: (capabilityVersionId: string) => client.get('/preconditions', { params: { capabilityVersionId } }),
  create: (data: { capabilityVersionId: string; name: string; command: string; enabled?: boolean }) =>
    client.post('/preconditions', data),
  update: (id: string, data: { name?: string; command?: string; enabled?: boolean }) => client.put(`/preconditions/${id}`, data),
  delete: (id: string) => client.delete(`/preconditions/${id}`),
};

export const approvalApi = {
  list: (releaseId: string) => client.get('/approvals', { params: { releaseId } }),
  listPending: () => client.get('/approvals/pending'),
  vote: (releaseId: string, data: { decision: 'approve' | 'reject'; comment?: string }) =>
    client.post(`/releases/${releaseId}/approvals`, data),
};

export const compilerApi = {
  compile: (manifest: unknown, options?: { capabilityContext?: string; constraints?: string[] }) =>
    client.post('/compiler/compile', { manifest, ...options }),
  decompile: (manifest: unknown) => client.post('/compiler/decompile', { manifest }),
};

export const selfEvolveApi = {
  getState: (capabilityId: string) => client.get(`/capabilities/${capabilityId}/self-evolve`),
  runCycle: (capabilityId: string) => client.post(`/capabilities/${capabilityId}/self-evolve/run`),
};

export const manifestApi = {
  get: async (versionId: string): Promise<{ data: CapabilityManifestResponse }> => {
    const r = await client.get<unknown>(`/capability-versions/${versionId}/manifest`);
    return { data: parseCapabilityManifest(r.data) };
  },
  getByHash: (hash: string) => client.get(`/manifests/${hash}`),
  create: (data: unknown) => client.post('/manifests', data),
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
  list: () => client.get('/webhooks'),
  create: (data: { organizationId: string; label: string; url: string; events: string[] }) => client.post('/webhooks', data),
  update: (id: string, data: { url?: string; events?: string[]; active?: boolean }) => client.put(`/webhooks/${id}`, data),
  delete: (id: string) => client.delete(`/webhooks/${id}`),
};

export const apiKeyApi = {
  list: () => client.get('/api-keys'),
  create: (data: { name: string; role: string; userId: string }) => client.post('/api-keys', data),
  revoke: (id: string) => client.delete(`/api-keys/${id}`),
};

export const userApi = {
  list: () => client.get('/users'),
  updateRole: (id: string, role: string) => client.put(`/users/${id}/role`, { role }),
  me: () => client.get('/users/me'),
};

export const featureFlagApi = {
  list: () => client.get('/feature-flags'),
  update: (key: string, data: { value: unknown; enabled?: boolean }) => client.put(`/feature-flags/${key}`, data),
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
  list: (organizationId: string) =>
    client.get(`/orgs/${organizationId}/signing-keys`).then((r) => r.data),
  upload: (organizationId: string, label: string, publicKeyPem: string) =>
    client.post(`/orgs/${organizationId}/signing-keys`, { organizationId, label, publicKeyPem }).then((r) => r.data),
};

export const evalSuiteApi = {
  list: async (capabilityId?: string): Promise<EvalSuite[]> => {
    const r = await client.get<unknown>(`/eval-suites${capabilityId ? `?capabilityId=${capabilityId}` : ''}`);
    return parseEvalSuites(r.data);
  },
  get: (id: string) => client.get(`/eval-suites/${id}`).then((r) => r.data),
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

export const vaultApi = {
  listSecrets: (organizationId: string) =>
    client.get(`/vault/secrets?organizationId=${encodeURIComponent(organizationId)}`).then((r) => r.data),
  listKeys: async (): Promise<VaultKeyringEntry[]> => {
    const r = await client.get<unknown>('/vault/keys');
    return parseVaultKeyring(r.data);
  },
  rotateKey: (label: string, reencrypt = true) =>
    client.post('/vault/keys/rotate', { label, reencrypt }).then((r) => r.data),
  writeSecret: (organizationId: string, name: string, value: string) =>
    client.post('/vault/secrets', { organizationId, name, value }).then((r) => r.data),
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
    provider: 'openai' | 'anthropic' | 'bedrock' | 'custom';
    temperature?: number;
    baseUrl?: string;
    apiKey?: string;
  }) => client.post<PlaygroundRun>('/playground/complete', data).then((r) => r.data),
  sweep: (data: {
    base: {
      prompt: string;
      model: string;
      provider: 'openai' | 'anthropic' | 'bedrock' | 'custom';
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
  rollup: (days = 30) =>
    client
      .get<{ days: number; items: Array<{ day: string; tokens: number; cost: number; runs: number }> }>(
        `/traces/rollup`,
        { params: { days } },
      )
      .then((r) => r.data),
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
  list: (params?: { resource?: string; action?: string }) => client.get('/audit', { params }),
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
  provider?: string;
  issuer?: string;
  clientId?: string;
  scopes?: string;
  audience?: string | null;
  groupsClaim?: string;
  emailClaim?: string;
  nameClaim?: string;
  enabled?: boolean;
}

export const teamApi = {
  list: () => client.get<{ items: TeamSummary[] }>('/teams').then((r) => r.data),
  create: (data: { name: string; slug: string; description?: string }) =>
    client.post<TeamSummary>('/teams', data).then((r) => r.data),
  addMember: (teamId: string, data: { userId: string; role?: TeamMember['role'] }) =>
    client.post<TeamMember>(`/teams/${teamId}/members`, data).then((r) => r.data),
  removeMember: (teamId: string, userId: string) =>
    client.delete<unknown>(`/teams/${teamId}/members/${userId}`).then((r) => r.data),
  ssoGet: () => client.get<SsoConfigView>('/auth/oidc/config').then((r) => r.data),
  ssoSet: (data: {
    provider: string;
    issuer: string;
    clientId: string;
    clientSecret: string;
    scopes?: string;
    audience?: string;
    groupsClaim?: string;
    emailClaim?: string;
    nameClaim?: string;
  }) => client.post<{ status: string; provider: string }>('/auth/oidc/config', data).then((r) => r.data),
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
