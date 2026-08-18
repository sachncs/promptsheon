import axios from 'axios';

const client = axios.create({
  baseURL: '/api',
  headers: { 'Content-Type': 'application/json' },
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
