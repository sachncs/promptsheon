import { loadSettings } from "./settings.js";

const DEFAULT_TIMEOUT_MS = 8000;
const RETRYABLE_STATUSES = new Set([0, 408, 425, 429, 500, 502, 503, 504]);

function buildUrl(path) {
  const settings = loadSettings();
  if (!path.startsWith("/")) path = "/" + path;
  if (settings.apiBase) {
    return settings.apiBase.replace(/\/$/, "") + path;
  }
  return path;
}

function buildQuery(params) {
  if (!params) return "";
  const qs = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === "") continue;
    qs.set(key, String(value));
  }
  const out = qs.toString();
  return out ? `?${out}` : "";
}

export async function sequential(thunks, { delayMs = 25, maxParallel = 2 } = {}) {
  const results = new Array(thunks.length);
  let cursor = 0;
  async function worker() {
    while (cursor < thunks.length) {
      const i = cursor++;
      try { results[i] = await thunks[i](); }
      catch (e) { results[i] = { ok: false, status: 0, error: String(e?.message || e) }; }
      if (cursor < thunks.length) await new Promise((r) => setTimeout(r, delayMs + Math.random() * 20));
    }
  }
  const workers = Array.from({ length: Math.min(maxParallel, thunks.length) }, worker);
  await Promise.all(workers);
  return results;
}

function buildHeaders(extra, hasBody, explicitKey) {
  const settings = loadSettings();
  const headers = { Accept: "application/json", ...(extra || {}) };
  const key = explicitKey !== undefined ? explicitKey : settings.apiKey;
  if (key) headers.Authorization = "Bearer " + key;
  if (hasBody && !headers["Content-Type"]) headers["Content-Type"] = "application/json";
  return headers;
}

async function sleep(ms) {
  return new Promise((r) => setTimeout(r, ms));
}

export async function apiFetch(path, options = {}) {
  const settings = loadSettings();
  const url = buildUrl(path);
  const explicitKey = options.apiKey;
  const maxAttempts = options.retry === false ? 1 : 3;
  let lastResult = null;
  for (let attempt = 0; attempt < maxAttempts; attempt++) {
    const controller = new AbortController();
    const timer = window.setTimeout(() => controller.abort(), options.timeout || DEFAULT_TIMEOUT_MS);
    try {
      const response = await fetch(url, {
        method: options.method || "GET",
        headers: buildHeaders(options.headers, options.body !== undefined, explicitKey),
        body: options.body !== undefined ? (typeof options.body === "string" ? options.body : JSON.stringify(options.body)) : undefined,
        signal: controller.signal,
        credentials: "omit"
      });
      const contentType = response.headers.get("content-type") || "";
      const text = await response.text();
      const payload = text && contentType.includes("application/json") ? safeJson(text) : text;
      const errorMessage = (payload && typeof payload === "object" && payload.error) || response.statusText || `HTTP ${response.status}`;
      const result = {
        ok: response.ok,
        status: response.status,
        error: response.ok ? null : errorMessage,
        data: response.ok ? payload : null,
        retryable: RETRYABLE_STATUSES.has(response.status),
        path,
        attempt
      };
      lastResult = result;
      if (result.ok) return result;
      if (!result.retryable || attempt === maxAttempts - 1) return result;
      const wait = Math.min(2000, 250 * Math.pow(2, attempt) + Math.random() * 100);
      await sleep(wait);
    } catch (error) {
      lastResult = {
        ok: false,
        status: 0,
        error: error.name === "AbortError" ? "request timed out" : "network error",
        data: null,
        retryable: true,
        path,
        attempt
      };
      if (attempt === maxAttempts - 1) return lastResult;
      const wait = Math.min(2000, 250 * Math.pow(2, attempt) + Math.random() * 100);
      await sleep(wait);
    } finally {
      window.clearTimeout(timer);
    }
  }
  return lastResult;
}

function safeJson(text) {
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

export async function apiGet(path) {
  return apiFetch(path);
}

export async function apiPost(path, body) {
  return apiFetch(path, { method: "POST", body });
}

export async function apiPut(path, body) {
  return apiFetch(path, { method: "PUT", body });
}

export async function apiDelete(path) {
  return apiFetch(path, { method: "DELETE" });
}

export async function getHealth() {
  return apiFetch("/health", { retry: false });
}

export async function getReady() {
  return apiFetch("/ready", { retry: false });
}

export async function getMetricsSummary() {
  return apiFetch("/api/v1/metrics/summary");
}

export async function listWorkspaces() {
  return apiFetch("/api/v1/workspaces");
}

// createWorkspace was an admin bootstrap helper; the dashboard
// does not create workspaces. Reintroduce when the admin
// workspace-creation flow lands.

export async function listProjects(workspaceId) {
  return apiFetch(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/projects`);
}

export async function createProject(workspaceId, name, description) {
  return apiPost(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/projects`, { name, description: description || undefined });
}

export async function listCapabilities(projectId) {
  return apiFetch(`/api/v1/projects/${encodeURIComponent(projectId)}/capabilities`);
}

// listAllCapabilities was the catalog search preview endpoint;
// the dashboard lists capabilities per-project today, not
// cross-project. Reintroduce when the catalog tab ships.

export async function createCapability(projectId, payload) {
  return apiPost(`/api/v1/projects/${encodeURIComponent(projectId)}/capabilities`, payload);
}

export async function getCapability(id) {
  return apiFetch(`/api/v1/capabilities/${encodeURIComponent(id)}`);
}

export async function listReleases(capabilityId) {
  return apiFetch(`/api/v1/capabilities/${encodeURIComponent(capabilityId)}/releases`);
}

export async function getRelease(id) {
  return apiFetch(`/api/v1/releases/${encodeURIComponent(id)}`);
}

export async function getReleaseApproval(id) {
  return apiFetch(`/api/v1/releases/${encodeURIComponent(id)}/approval`);
}

export async function rollbackRelease(id) {
  return apiFetch(`/api/v1/releases/${encodeURIComponent(id)}/rollback`, { method: "POST" });
}

// invokeRelease was the eval-loop shortcut; the harness runner
// drives invocations server-side today and the dashboard
// triggers evals via runEval, not direct invoke. Reintroduce
// when an admin "try it" button lands.

export async function getCapabilityContract(id) {
  return apiFetch(`/api/v1/capabilities/${encodeURIComponent(id)}/contract`);
}

export async function updateCapabilityContract(id, payload) {
  return apiPut(`/api/v1/capabilities/${encodeURIComponent(id)}/contract`, payload);
}

export async function getCapabilityReputation(id) {
  return apiFetch(`/api/v1/capabilities/${encodeURIComponent(id)}/reputation`);
}

export async function updateCapability(id, payload) {
  return apiPut(`/api/v1/capabilities/${encodeURIComponent(id)}`, payload);
}

export async function deleteCapability(id) {
  return apiDelete(`/api/v1/capabilities/${encodeURIComponent(id)}`);
}

export async function getCapabilityDiff(id, fromVersion, toVersion) {
  return apiFetch(`/api/v1/capabilities/${encodeURIComponent(id)}/diff?from=${fromVersion}&to=${toVersion}`);
}

export async function listVersions(capabilityId) {
  return apiFetch(`/api/v1/capabilities/${encodeURIComponent(capabilityId)}/versions`);
}

// getLatestVersion was a quick "show me the latest" shortcut;
// the capability-detail view fetches the full version list
// and picks the latest from it. Reintroduce only if a route
// starts depending on the single-version endpoint.

export async function createVersion(capabilityId, payload) {
  return apiPost(`/api/v1/capabilities/${encodeURIComponent(capabilityId)}/versions`, payload);
}

export async function updateSelfEvolveConfig(id, payload) {
  return apiPut(`/api/v1/capabilities/${encodeURIComponent(id)}/self-evolve`, payload);
}

export async function listExecutions(versionId) {
  return apiFetch(`/api/v1/versions/${encodeURIComponent(versionId)}/executions`);
}

export async function createExecution(versionId, body) {
  return apiPost(`/api/v1/versions/${encodeURIComponent(versionId)}/executions`, body);
}

export async function getReleaseCreation(versionId, environment) {
  return apiPost(`/api/v1/versions/${encodeURIComponent(versionId)}/releases`, { environment });
}

// getWorkspaceObservation was the per-workspace rollup
// dashboard endpoint; the rollup pipeline was unwired (see
// CHANGELOG [Unreleased]), so this would 200 with an empty
// summary forever. Reintroduce when the rollup job ships.

export async function listAlertRules() {
  return apiFetch("/api/v1/alerts/rules");
}

export async function createAlertRule(payload) {
  return apiPost("/api/v1/alerts/rules", payload);
}

// updateAlertRule was the edit-existing-alert path; the
// dashboard today only creates + deletes alerts. Reintroduce
// when the alerts page grows an edit form.

export async function deleteAlertRule(id) {
  return apiDelete(`/api/v1/alerts/rules/${encodeURIComponent(id)}`);
}

export async function listNotificationGroups() {
  return apiFetch("/api/v1/alerts/notifications");
}

export async function createNotificationGroup(payload) {
  return apiPost("/api/v1/alerts/notifications", payload);
}

export async function resolveAlert(id) {
  return apiPut(`/api/v1/alerts/active/${encodeURIComponent(id)}/resolve`);
}

export async function listWebhooks() {
  return apiFetch("/api/v1/webhooks");
}

export async function createWebhook(payload) {
  return apiPost("/api/v1/webhooks", payload);
}

export async function deleteWebhook(id) {
  return apiDelete(`/api/v1/webhooks/${encodeURIComponent(id)}`);
}

export async function listVaultKeys() {
  return apiFetch("/api/v1/vault/keys");
}

export async function saveVaultKey(payload) {
  return apiPost("/api/v1/vault/keys", payload);
}

export async function deleteVaultKey(id) {
  return apiDelete(`/api/v1/vault/keys/${encodeURIComponent(id)}`);
}

// getProvider was the per-provider detail endpoint; the
// dashboard today only renders the providers list. Reintroduce
// when a provider-detail page ships.

export async function testProvider(name, model) {
  return apiPost(`/api/v1/providers/${encodeURIComponent(name)}/test`, { model });
}

export async function listUsers(limit = 200) {
  return apiFetch(`/api/v1/users?limit=${limit}`);
}

export async function createUser(payload) {
  return apiPost("/api/v1/users", payload);
}

// updateUser was the user-edit path; the admin users page
// today only lists + creates + deletes users. Reintroduce
// when the user-edit form lands.

export async function deleteUser(id) {
  return apiDelete(`/api/v1/users/${encodeURIComponent(id)}`);
}

export async function compileReasoning(intent, workspaceId) {
  return apiPost("/api/v1/reasoning/compile", { intent, workspace_id: workspaceId || undefined });
}

export async function getTopCapabilities() {
  return apiFetch("/api/v1/metrics/top-capabilities");
}

export async function listAudit(options = {}) {
  const params = { limit: options.limit || 24 };
  if (options.action) params.action = options.action;
  if (options.resource) params.resource = options.resource;
  if (options.user_id) params.user_id = options.user_id;
  if (options.since) params.since = options.since;
  if (options.until) params.until = options.until;
  if (options.offset != null) params.offset = options.offset;
  return apiFetch(`/api/v1/audit${buildQuery(params)}`);
}

export async function verifyAuditChain() {
  return apiGet("/api/v1/audit/verify");
}

export async function exportAudit(format = "csv") {
  const headers = { Accept: "text/csv" };
  if (loadSettings().apiKey) headers.Authorization = "Bearer " + loadSettings().apiKey;
  const url = buildUrl(`/api/v1/audit/export?format=${encodeURIComponent(format)}`);
  const response = await fetch(url, { headers, credentials: "omit" });
  return { ok: response.ok, status: response.status, blob: response.ok ? await response.blob() : null, error: response.ok ? null : `HTTP ${response.status}` };
}

export async function listAlerts() {
  return apiFetch("/api/v1/alerts/active");
}

export async function listProviders() {
  return apiFetch("/api/v1/providers");
}

export async function setupBootstrap() {
  return apiPost("/api/v1/setup", {});
}

export async function voteRelease(releaseId, decision, reason) {
  return apiFetch(`/api/v1/releases/${encodeURIComponent(releaseId)}/votes`, {
    method: "POST",
    body: { identity: "ui", decision, reason: reason || undefined }
  });
}

export async function activateRelease(releaseId) {
  return apiFetch(`/api/v1/releases/${encodeURIComponent(releaseId)}/activate`, { method: "POST" });
}
