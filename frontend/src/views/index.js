import { renderOverview } from "./overview.js";
import { renderCapabilitiesList } from "./capabilities-list.js";
import { renderCapabilityDetail } from "./capability-detail.js";
import { renderAudit } from "./audit.js";
import { renderReleases } from "./releases.js";
import { renderObservability } from "./observability.js";
import { renderGuardrails } from "./guardrails.js";
import { renderEvaluations } from "./evaluations.js";
import { renderOperations } from "./operations/index.js";
import { renderLogs } from "./logs.js";
import { renderNotFound } from "./not-found.js";
import { renderWorkspaceDetail } from "./workspace-detail.js";
import { renderProjectDetail } from "./project-detail.js";
import { renderVersionDetail } from "./version-detail.js";
import { renderExecutionDetail } from "./execution-detail.js";
import { loadSettings } from "../settings.js";

const ROUTES = {
  "/": renderOverview,
  "/capabilities": renderCapabilitiesList,
  "/capabilities/{id}": renderCapabilityDetail,
  "/releases": renderReleases,
  "/audit": renderAudit,
  "/observability": renderObservability,
  "/guardrails": renderGuardrails,
  "/evaluations": renderEvaluations,
  "/logs": renderLogs,
  "/operations": renderOperations
};

const PAGE_TITLES = {
  "/": "Overview",
  "/capabilities": "Capabilities",
  "/capabilities/{id}": "Capability",
  "/releases": "Releases",
  "/audit": "Audit trail",
  "/observability": "Observability",
  "/guardrails": "Guardrails",
  "/evaluations": "Evaluations",
  "/logs": "Live logs",
  "/operations": "Operations"
};

function resolve(route) {
  if (ROUTES[route.path]) return ROUTES[route.path];
  if (route.path.startsWith("/capabilities/") && route.path !== "/capabilities") return renderCapabilityDetail;
  if (route.path.startsWith("/operations/")) return renderOperations;
  if (route.path.startsWith("/workspaces/") && route.path !== "/workspaces") return renderWorkspaceDetail;
  if (route.path.startsWith("/projects/") && route.path !== "/projects") return renderProjectDetail;
  if (route.path.startsWith("/versions/") && route.path !== "/versions") return renderVersionDetail;
  if (route.path.startsWith("/executions/") && route.path !== "/executions") return renderExecutionDetail;
  return null;
}

function titleFromRoute(route) {
  if (route.path === "/capabilities/{id}") return route.params.id ? `Capability ${route.params.id.slice(-8)}` : "Capability";
  if (route.path === "/operations/{tab}") return route.params.tab ? `Operations · ${prettyTab(route.params.tab)}` : "Operations";
  if (route.path === "/workspaces/{id}") return route.params.id ? `Workspace ${route.params.id.slice(-8)}` : "Workspace";
  if (route.path === "/projects/{id}") return route.params.id ? `Project ${route.params.id.slice(-8)}` : "Project";
  if (route.path === "/versions/{id}") return route.params.id ? `Version v${route.params.id.slice(-8)}` : "Version";
  if (route.path === "/executions/{id}") return route.params.id ? `Execution ${route.params.id.slice(-8)}` : "Execution";
  return null;
}

function prettyTab(t) {
  return ({ alerts: "Alerts", webhooks: "Webhooks", vault: "Vault", providers: "Providers", users: "Users", reasoning: "Reasoning" })[t] || t;
}

export async function renderView(route) {
  const view = window.document.getElementById("view");
  const title = window.document.getElementById("page-title");
  if (!view) return;
  const matched = resolve(route);
  if (!matched) {
    view.innerHTML = renderNotFound(route);
    if (title) title.textContent = titleFromRoute(route) || "Promptsheon";
    return;
  }
  const result = matched(route);
  if (result && typeof result.then === "function") {
    const html = await result;
    // Renderers that mutate the DOM (e.g. overview) return "" to avoid clobbering bindings.
    if (typeof html === "string" && html && view.innerHTML.trim() === "") {
      view.innerHTML = html;
    }
  } else if (typeof result === "string" && result) {
    view.innerHTML = result;
  }
  if (title) title.textContent = PAGE_TITLES[route.path] || titleFromRoute(route) || "Promptsheon";
}

// requiresKey was a shared check used by an earlier router hook;
// the connect-prompt logic inlines its own loadSettings().apiKey
// test today. Reintroduce when the hook comes back.

export function renderConnectPrompt(message) {
  return `<section class="panel p-8 text-center">
    <div class="mx-auto grid h-12 w-12 place-items-center rounded-2xl bg-paper text-muted"><svg class="h-6 w-6 fill-none stroke-current stroke-2"><use href="#icon-key"/></svg></div>
    <h1 class="mt-5 text-[1.4rem] font-bold tracking-[-.04em]">Connect the Promptsheon API</h1>
    <p class="mt-2 text-[.78rem] text-muted">${message || 'Open <span class="font-bold text-ink">Connection</span> in the sidebar to paste an API key.'}</p>
    <div class="mt-5 flex justify-center gap-2">
      <button data-open-settings class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-settings"/></svg>Open Connection</button>
    </div>
  </section>`;
}
