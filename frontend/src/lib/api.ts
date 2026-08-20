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

export function subscribeSSE(channel: string, onEvent: (event: unknown) => void): () => void {
  if (typeof window === 'undefined') return () => undefined;
  const eventSource = new EventSource(`/api/events/${channel}`);
  eventSource.onmessage = (e) => onEvent(JSON.parse(e.data));
  eventSource.onerror = () => {
    eventSource.close();
    setTimeout(() => subscribeSSE(channel, onEvent), 3000);
  };
  return () => eventSource.close();
}

export const workspaceApi = {
  list: (page = 1) => client.get('/workspaces', { params: { page } }),
  get: (id: string) => client.get(`/workspaces/${id}`),
  create: (data: { name: string; organization?: string }) => client.post('/workspaces', data),
  update: (id: string, data: { name?: string; organization?: string }) => client.put(`/workspaces/${id}`, data),
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
  supersede: (id: string) => client.put(`/releases/${id}/supersede`),
};

export const executionApi = {
  list: (capabilityVersionId: string) => client.get('/executions', { params: { capabilityVersionId } }),
  get: (id: string) => client.get(`/executions/${id}`),
  execute: (data: { manifestHash: string; inputs: Record<string, unknown>; environment?: string; traceId?: string }) =>
    client.post('/executions', data),
};

export const invokeApi = {
  invoke: (data: { capabilityVersionId: string; inputs: Record<string, unknown>; environment?: string; traceId?: string }) =>
    client.post('/invoke', data),
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
};

export const featureFlagApi = {
  list: () => client.get('/feature-flags'),
  update: (key: string, data: { value: unknown; enabled?: boolean }) => client.put(`/feature-flags/${key}`, data),
};

export const auditApi = {
  list: (params?: { resource?: string; action?: string }) => client.get('/audit', { params }),
};