import { loadSettings, saveSettings, clearSettings } from "./settings.js";

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

export async function getVersion() {
  return apiFetch("/api/v1/version", { retry: false });
}

export async function getMetricsSummary() {
  return apiFetch("/api/v1/metrics/summary");
}

export async function listWorkspaces() {
  return apiFetch("/api/v1/workspaces");
}

export async function createWorkspace(name, organization) {
  return apiPost("/api/v1/workspaces", { name, organization: organization || undefined });
}

export async function listProjects(workspaceId) {
  return apiFetch(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/projects`);
}

export async function createProject(workspaceId, name, description) {
  return apiPost(`/api/v1/workspaces/${encodeURIComponent(workspaceId)}/projects`, { name, description: description || undefined });
}

export async function listCapabilities(projectId) {
  return apiFetch(`/api/v1/projects/${encodeURIComponent(projectId)}/capabilities`);
}

export async function listAllCapabilities() {
  return apiFetch(`/api/v1/catalog/capabilities`);
}

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

export async function listAudit(limit = 12) {
  return apiFetch(`/api/v1/audit?limit=${limit}`);
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
