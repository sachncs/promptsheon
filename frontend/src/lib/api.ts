import axios from 'axios';
import { getSession } from './session';

const client = axios.create({
  baseURL: '/api',
  headers: { 'Content-Type': 'application/json' },
});

client.interceptors.request.use((config) => {
  const session = getSession();
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
    const message = err.response?.data?.error?.message ?? err.message;
    return Promise.reject(new Error(message));
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
  let cancelled = false;
  let active: EventSource | null = null;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  const open = () => {
    if (cancelled) return;
    active = new EventSource(`/api/events/${channel}`);
    active.onmessage = (e) => onEvent(JSON.parse(e.data));
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
  list: async (page = 1): Promise<{ data: WorkspaceRow[] }> => {
    const r = await client.get<{ items?: WorkspaceRow[]; total?: number }>('/workspaces', {
      params: { page },
    });
    return { data: unwrapList<WorkspaceRow>(r.data) };
  },
  get: (id: string): Promise<{ data: WorkspaceRow }> =>
    client.get(`/workspaces/${id}`).then((r) => ({ data: r.data as WorkspaceRow })),
  create: (data: { name: string; organization?: string }): Promise<{ data: WorkspaceRow }> =>
    client.post('/workspaces', data).then((r) => ({ data: r.data as WorkspaceRow })),
  update: (id: string, data: { name?: string; organization?: string }): Promise<{ data: WorkspaceRow }> =>
    client.put(`/workspaces/${id}`, data).then((r) => ({ data: r.data as WorkspaceRow })),
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
  list: (projectId: string) => client.get('/capabilities', { params: { projectId } }),
  get: (id: string) => client.get(`/capabilities/${id}`),
  create: (data: { projectId: string; name: string; description?: string }) => client.post('/capabilities', data),
  update: (id: string, data: { name?: string; description?: string }) => client.put(`/capabilities/${id}`, data),
  delete: (id: string) => client.delete(`/capabilities/${id}`),
};

export const versionApi = {
  list: (capabilityId: string) => client.get('/capability-versions', { params: { capabilityId } }),
  get: (id: string) => client.get(`/capability-versions/${id}`),
  create: (data: { capabilityId: string; version: number; manifest: string; manifestHash: string; createdBy?: string }) =>
    client.post('/capability-versions', data),
};

export const releaseApi = {
  list: (capabilityId: string) => client.get('/releases', { params: { capabilityId } }),
  get: (id: string) => client.get(`/releases/${id}`),
  create: (data: { capabilityId: string; capabilityVersion: number; capabilityVersionId: string | null; manifest: string; environment: string }) =>
    client.post('/releases', data),
  activate: (id: string) => client.put(`/releases/${id}/activate`),
  canary: (id: string, percent: number) => client.put(`/releases/${id}/canary`, { percent }),
  supersede: (id: string) => client.put(`/releases/${id}/supersede`),
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
    void fetch(url, {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        accept: 'text/event-stream',
      },
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
  invoke: (data: { capabilityVersionId: string; inputs: Record<string, unknown>; environment?: string; traceId?: string }) =>
    client.post('/invoke', data),
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
  get: (id: string) => client.get(`/eval-runs/${id}`),
  create: (data: { releaseId: string; datasetId: string; scorer: string }) => client.post('/eval-runs', data),
  getResults: (id: string) => client.get(`/eval-runs/${id}/results`),
};

export const alertApi = {
  listRules: () => client.get('/alert-rules'),
  createRule: (data: { name: string; type: string; severity: string; threshold?: number; window?: number }) =>
    client.post('/alert-rules', data),
  deleteRule: (id: string) => client.delete(`/alert-rules/${id}`),
  listAlerts: () => client.get('/alerts'),
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
  vote: (releaseId: string, data: { decision: 'approve' | 'reject'; comment?: string }) =>
    client.post(`/releases/${releaseId}/approvals`, data),
};

export const compilerApi = {
  compile: (prompt: string) => client.post('/compiler/compile', { prompt }),
  decompile: (manifest: string) => client.post('/compiler/decompile', { manifest }),
};

export const selfEvolveApi = {
  getState: (capabilityId: string) => client.get(`/capabilities/${capabilityId}/self-evolve`),
  runCycle: (capabilityId: string) => client.post(`/capabilities/${capabilityId}/self-evolve/run`),
};

export const manifestApi = {
  get: (versionId: string) => client.get(`/capability-versions/${versionId}/manifest`),
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
  create: (data: { url: string; events: string[] }) => client.post('/webhooks', data),
  update: (id: string, data: { url?: string; events?: string[]; active?: boolean }) => client.put(`/webhooks/${id}`, data),
  delete: (id: string) => client.delete(`/webhooks/${id}`),
};

export const apiKeyApi = {
  list: () => client.get('/api-keys'),
  create: (data: { name: string; role: string }) => client.post('/api-keys', data),
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

export const repoApi = {
  list: (workspaceId: string) => client.get(`/repos?workspaceId=${encodeURIComponent(workspaceId)}`).then((r) => r.data),
  get: (id: string) => client.get(`/repos/${id}`).then((r) => r.data),
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
  listBranches: (repoId: string) => client.get(`/repos/${repoId}/branches`).then((r) => r.data),
  listTags: (repoId: string) => client.get(`/repos/${repoId}/tags`).then((r) => r.data),
  listContents: (repoId: string, ref = 'main') => client.get(`/repos/${repoId}/contents?ref=${encodeURIComponent(ref)}`).then((r) => r.data),
  getFile: (repoId: string, path: string, ref = 'main') => client.get(`/repos/${repoId}/contents/${encodeURIComponent(path)}?ref=${encodeURIComponent(ref)}`),
  putFile: (repoId: string, path: string, content: string, ref = 'main') =>
    client.put(`/repos/${repoId}/contents/${encodeURIComponent(path)}?ref=${encodeURIComponent(ref)}`, { path, content, ref }).then((r) => r.data),
  commit: (repoId: string, ref: string, message: string, parents?: string[]) =>
    client.post(`/repos/${repoId}/commits`, { ref, message, parents }).then((r) => r.data),
  listCommits: (repoId: string, ref: string) => client.get(`/repos/${repoId}/commits?ref=${encodeURIComponent(ref)}`).then((r) => r.data),
  listMRs: (repoId: string, status?: string) =>
    client.get(`/repos/${repoId}/merge-requests${status ? `?status=${status}` : ''}`).then((r) => r.data),
  openMR: (input: {
    repositoryId: string;
    title: string;
    description?: string;
    sourceBranch: string;
    targetBranch: string;
    sourceCommitOid: string;
  }) => client.post(`/repos/${input.repositoryId}/merge-requests`, input).then((r) => r.data),
  getMR: (id: string) => client.get(`/merge-requests/${id}`).then((r) => r.data),
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
  list: (capabilityId?: string) =>
    client.get(`/eval-suites${capabilityId ? `?capabilityId=${capabilityId}` : ''}`).then((r) => r.data),
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
  listKeys: () => client.get('/vault/keys').then((r) => r.data),
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
  forOrg: (organizationId: string, days = 30) =>
    client.get(`/analytics/cost?organizationId=${encodeURIComponent(organizationId)}&days=${days}`).then((r) => r.data),
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
  q: (q: string, type?: string) =>
    client.get(`/search?q=${encodeURIComponent(q)}${type ? `&type=${encodeURIComponent(type)}` : ''}`).then((r) => r.data),
};