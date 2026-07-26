import { renderOverview } from "./overview.js";
import { renderCapabilitiesListPlaceholder, renderCapabilityDetailPlaceholder, renderReleasesPlaceholder, renderAuditPlaceholder, renderObservabilityPlaceholder, renderGuardrailsPlaceholder, renderEvaluationsPlaceholder, renderLogsPlaceholder, renderOperationsPlaceholder } from "./_placeholder.js";
import { renderNotFound } from "./not-found.js";

const ROUTES = {
  "/": renderOverview,
  "/capabilities": renderCapabilitiesListPlaceholder,
  "/capabilities/{id}": renderCapabilityDetailPlaceholder,
  "/releases": renderReleasesPlaceholder,
  "/audit": renderAuditPlaceholder,
  "/observability": renderObservabilityPlaceholder,
  "/guardrails": renderGuardrailsPlaceholder,
  "/evaluations": renderEvaluationsPlaceholder,
  "/logs": renderLogsPlaceholder,
  "/operations": renderOperationsPlaceholder,
  "/operations/{tab}": renderOperationsPlaceholder
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
  "/operations": "Operations",
  "/operations/{tab}": "Operations"
};

function resolve(route) {
  if (ROUTES[route.path]) return ROUTES[route.path];
  if (route.path.startsWith("/capabilities/") && route.path !== "/capabilities") return renderCapabilityDetailPlaceholder;
  if (route.path.startsWith("/operations/") && route.path !== "/operations") return renderOperationsPlaceholder;
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
  if (matched) {
    const htmlOrPromise = matched(route);
    const out = htmlOrPromise && typeof htmlOrPromise.then === "function" ? await htmlOrPromise : htmlOrPromise;
    view.innerHTML = typeof out === "string" ? out : "";
  } else {
    view.innerHTML = renderNotFound(route);
  }
  if (title) title.textContent = PAGE_TITLES[route.path] || titleFromRoute(route) || "Promptsheon";
}
