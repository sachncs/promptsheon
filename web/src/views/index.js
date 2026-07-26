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
  return null;
}

function titleFromRoute(route) {
  if (route.path === "/capabilities/{id}") return route.params.id ? `Capability ${route.params.id.slice(-8)}` : "Capability";
  if (route.path === "/operations/{tab}") return route.params.tab ? `Operations · ${prettyTab(route.params.tab)}` : "Operations";
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
