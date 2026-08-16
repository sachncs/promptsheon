// src/views/overview.js — command-center landing page.
//
// Pulls from /health, /ready, /api/v1/metrics/summary, /api/v1/providers,
// /api/v1/alerts, /api/v1/audit, /api/v1/workspaces, the project lists,
// and the per-project capability + release lists. Renders metric cards,
// pending releases, audit activity, environment summary, and a catalog
// preview.
//
// Refactored to consume the ui.js primitives — every status pill,
// empty state, error panel, metric card, and panel header goes
// through the same source as every other view.

import * as api from "../api.js";
import { escape, formatCompact, formatInteger, formatMoney, formatRelative } from "../utils.js";
import { ownerName } from "../state/owners.js";
import { statusPill, pageHeader, panel, emptyState, errorState, metricCard, metricGrid, dataTable, skeletonStack } from "../ui.js";

const STATUS_LABELS = { active: "Production", approved: "Approved", pending: "Pending", superseded: "Superseded", rolled_back: "Rolled back" };
const STATUS_TONES  = { active: "good",    approved: "Approved", pending: "Pending", superseded: "Superseded", rolled_back: "Rolled back" };
const ENV_TONES     = { prod: "good", staging: "warn", dev: "neutral" };
const ACTION_TONES  = { delete: "danger", update: "warn", create: "good", activate: "good", rollback: "danger", vote: "neutral", invoke: "neutral", approve: "good", reject: "danger", resolve: "good" };

function actionTone(action) { return ACTION_TONES[action] || "neutral"; }
function statusLabel(status) { return STATUS_LABELS[status] || status || "Draft"; }
function statusTone(status) { return STATUS_TONES[status] || "neutral"; }
function envTone(env) { return ENV_TONES[env] || "neutral"; }

async function loadOverviewData(initialAuditFilter) {
  const out = {
    health: null, ready: null, metrics: null, providers: null,
    alerts: null, audit: null, auditFilter: initialAuditFilter || "all",
    workspaces: null, projects: [], capabilities: [], releases: [], capabilityMap: new Map(),
  };
  out.health = await api.getHealth();
  out.ready = await api.getReady();
  if (!out.health.ok && !out.ready.ok) return out;
  const auditOpts = { limit: 12 };
  if (initialAuditFilter && initialAuditFilter !== "all") auditOpts.action = initialAuditFilter;
  // Sequential fetch with small jitter to stay under the daemon's per-second rate cap.
  out.metrics = await api.getMetricsSummary();
  await delay(40);
  out.providers = await api.listProviders();
  await delay(40);
  out.alerts = await api.listAlerts();
  await delay(40);
  out.audit = await api.listAudit(auditOpts);
  await delay(40);
  out.workspaces = await api.listWorkspaces();
  if (out.workspaces.ok && out.workspaces.data?.length) {
    const w = out.workspaces.data[0];
    const projects = await api.listProjects(w.id);
    if (projects.ok) {
      out.projects = projects.data || [];
      const capLists = await api.sequential(
        out.projects.map((p) => () => api.listCapabilities(p.id)),
        { delayMs: 30, maxParallel: 1 },
      );
      out.capabilities = capLists.filter((r) => r.ok).flatMap((r) => r.data || []);
      const relLists = await api.sequential(
        out.capabilities.map((c) => () => api.listReleases(c.id)),
        { delayMs: 30, maxParallel: 1 },
      );
      out.releases = relLists.filter((r) => r.ok).flatMap((r) => r.data || []);
      out.capabilityMap = new Map(out.capabilities.map((c) => [c.id, c]));
    }
  }
  return out;
}

function delay(ms) {
  return new Promise((r) => setTimeout(r, ms + Math.random() * 30));
}

function renderMetrics(metrics) {
  if (!metrics || !metrics.ok || !metrics.data) {
    const msg = metrics?.status === 401 ? "Live counters unlock once an API key is set."
      : metrics?.status === 429 ? "Daemon is rate limiting requests. Slowing down."
      : metrics?.error ? `Live counters unavailable (${escape(metrics.error)}).`
      : "Live counters unavailable while the daemon warms up.";
    return errorState({ ...metrics, error: msg }, { prefix: "" });
  }
  const apiM = metrics.data.api_metrics || {};
  const llm  = metrics.data.llm_metrics || {};
  const rev  = metrics.data.review_metrics || {};
  const gd   = metrics.data.guardrail_metrics || {};
  const errorRate = apiM.error_rate || 0;
  const reqBadge = errorRate > 0 ? "warn" : "good";
  const reqLabel = errorRate > 0 ? "Attention" : "Healthy";
  const reviewBadge = (rev.pending_count || 0) > 0 ? "warn" : "good";
  const guardBadge = (gd.blocks || 0) > 0 ? "warn" : "good";
  return metricGrid([
    metricCard({ eyebrow: "API requests", icon: "pulse", value: formatCompact(apiM.total_requests), sub: reqLabel, tone: reqBadge }),
    metricCard({ eyebrow: "LLM spend",    icon: "rocket", value: formatMoney(llm.total_cost_usd), sub: `${formatCompact(llm.total_calls)} calls`, tone: "neutral" }),
    metricCard({ eyebrow: "Reviews",      icon: "scroll", value: formatInteger(rev.total_reviews), sub: `${formatInteger(rev.pending_count)} pending`, tone: reviewBadge }),
    metricCard({ eyebrow: "Guardrails",   icon: "shield", value: formatInteger((gd.passes || 0) + (gd.blocks || 0)), sub: `${formatInteger(gd.blocks || 0)} blocks`, tone: guardBadge }),
  ]);
}

function renderEnvironments(releases, capabilitiesById) {
  const envs = ["prod", "staging", "dev"];
  return envs.map((env) => {
    const list = releases.filter((r) => r.environment === env);
    const active = list.find((r) => r.status === "active");
    const pending = list.filter((r) => r.status === "pending").length;
    const cap = active ? capabilitiesById.get(active.capability_id) : null;
    const label = active
      ? `${cap?.name || "capability"} v${active.capability_version}`
      : pending > 0 ? `${pending} pending review` : "no active release";
    const tone = envTone(env);
    const status = active ? "Active" : pending > 0 ? "Pending" : "Idle";
    return `<div class="flex items-center gap-3 rounded-xl bg-paper p-3">
      <span class="grid h-9 w-9 shrink-0 place-items-center rounded-[10px] bg-white text-[#5c5e63]"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-layers"/></svg></span>
      <span class="min-w-0 flex-1"><span class="flex items-center gap-2 text-[.75rem] font-bold">${escape(env)} <span class="mono text-[.58rem] font-medium text-muted">${escape(list.length)} total</span></span><span class="mt-1 block text-[.64rem] text-muted">${escape(label)}</span></span>
      ${statusPill(status, tone)}
    </div>`;
  }).join("");
}

function renderPending(releases, capabilitiesById) {
  const pending = releases.filter((r) => r.status === "pending");
  if (!pending.length) {
    return `<div class="mt-5 rounded-xl border border-dashed border-white/8 bg-white/[.04] p-5 text-center">
      <p class="text-[.78rem] font-bold text-white">Nothing waiting on you</p>
      <p class="mt-1 text-[.66rem] text-[#9a9da4]">All releases are either approved or shipped.</p>
    </div>`;
  }
  return `<div class="mt-5 space-y-2">${pending.slice(0, 6).map((r) => {
    const cap = capabilitiesById.get(r.capability_id);
    const name = cap?.name || "Unknown capability";
    const initials = (name || "?").split(/\s+/).filter(Boolean).slice(0, 2).map((p) => (p[0] || "").toUpperCase()).join("");
    return `<button type="button" data-open-release="${escape(r.id)}" class="flex w-full items-center gap-3 rounded-xl border border-white/8 bg-white/[.045] p-3 text-left transition hover:bg-white/[.09]">
      <span class="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-[#30323a] text-[.65rem] font-bold text-[#c5f06e]">${escape(initials)}</span>
      <span class="min-w-0 flex-1"><span class="block truncate text-[.75rem] font-bold">${escape(name)}</span><span class="mt-1 block text-[.65rem] text-[#898c94]">v${escape(r.capability_version)} · ${escape(r.environment)} · created ${escape(formatRelative(r.created_at))}</span></span>
      ${statusPill("Review", "warn")}
      <svg class="h-3.5 w-3.5 shrink-0 fill-none stroke-[#777a83] stroke-2"><use href="#icon-arrow-right"/></svg>
    </button>`;
  }).join("")}</div>`;
}

function renderAudit(audit, filter) {
  if (!audit || !audit.ok || !audit.data) {
    if (audit?.status === 429) return errorState({ ...audit, error: "Audit log rate-limited. Retrying automatically." });
    return errorState({ ...audit, error: audit?.error ? `Live activity unavailable (${audit.error}).` : "Live activity unavailable." });
  }
  const all = audit.data || [];
  const rows = all.slice(0, 12);
  if (!rows.length) return emptyState("No activity yet. Create a capability or release to populate the audit trail.", { icon: "icon-pulse" });
  const chips = ["all", "create", "update", "delete", "activate", "rollback"].map((action) => {
    const active = (filter || "all") === action;
    return `<button type="button" data-audit-filter="${escape(action)}" class="chip ${active ? "active" : ""}">${escape(action)}</button>`;
  }).join("");
  const list = rows.map((entry) => {
    const details = entry.details || {};
    const target = (entry.resource || "").split(":").pop();
    const label = details.name || target || entry.resource;
    const subject = (entry.user_id || "api") === "api" ? "System" : entry.user_id;
    return `<div class="flex gap-3">
      <span class="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-paper text-[#5c5e63]"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-pulse"/></svg></span>
      <div class="min-w-0 flex-1"><p class="text-[.74rem] leading-5"><span class="font-bold">${escape(subject)}</span> · <span class="font-semibold">${escape(entry.action)}</span> · <span class="font-bold">${escape(label)}</span></p><p class="mt-0.5 text-[.63rem] text-muted">${escape(formatRelative(entry.timestamp))} <span class="mx-1 text-[#c1c2bd]">·</span> <span class="mono">${escape(entry.resource)}</span></p></div>
      ${statusPill(entry.action, actionTone(entry.action))}
    </div>`;
  }).join("");
  return `<div class="mt-5 space-y-3">${list}</div>
  <div class="mt-4 chip-group" data-audit-chips>${chips}</div>
  <p class="mt-2 text-[.62rem] text-muted">Showing ${rows.length} of ${all.length} entries. Open <a class="font-bold text-ink hover:text-accent" href="#/audit${filter && filter !== "all" ? `?action=${encodeURIComponent(filter)}` : ""}">Audit trail →</a> for full history with filters.</p>`;
}

function renderCapabilities(capabilities, releases) {
  if (!capabilities.length) {
    return emptyState("No capabilities yet. Use the New capability button above to add the first one.", { icon: "icon-layers" });
  }
  const byCap = new Map();
  for (const r of releases) {
    if (!byCap.has(r.capability_id)) byCap.set(r.capability_id, []);
    byCap.get(r.capability_id).push(r);
  }
  return dataTable({
    columns: [
      { key: "name", label: "Capability", render: (cap) => {
          return `<div class="flex min-w-0 items-center gap-3">
            <span class="grid h-9 w-9 shrink-0 place-items-center rounded-[10px] bg-[#e9ecff] text-blue"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-spark"/></svg></span>
            <span class="min-w-0"><a href="#/capabilities/${escape(cap.id)}" class="block truncate text-[.78rem] font-bold hover:underline">${escape(cap.name)}</a><span class="mt-1 block truncate text-[.64rem] text-muted">${escape(cap.description || (cap.tags && cap.tags.join(" · ")) || "No description")}</span></span>
          </div>`;
        },
      },
      { key: "version", label: "Version", render: (cap) => {
          const rels = byCap.get(cap.id) || [];
          const version = rels.reduce((m, r) => Math.max(m, r.capability_version || 0), 0) || 1;
          return `<span class="mono text-[.68rem] text-[#62656a]">v${escape(version)}</span>`;
        },
      },
      { key: "status", label: "Status", render: (cap) => {
          const rels = byCap.get(cap.id) || [];
          const release = rels.find((r) => r.status === "active") || rels.find((r) => r.status === "pending") || rels[0];
          const tone = release ? envTone(release.environment) : "neutral";
          return statusPill(statusLabel(release?.status), tone);
        },
      },
      { key: "reliability", label: "Reliability", render: () => `<span class="text-[.74rem] font-bold">— <span class="block text-[.62rem] font-medium text-muted">no evals yet</span></span>` },
      { key: "owner", label: "Owner", render: (cap) => `<span class="text-[.72rem] text-[#686b70]">${escape(ownerName(cap.owner))}</span>` },
      { key: "go", label: "", align: "right", render: (cap) => `<a href="#/capabilities/${escape(cap.id)}" class="icon-button !h-7 !w-7 !border-0 !bg-transparent" aria-label="Open"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.8]"><use href="#icon-arrow-right"/></svg></a>` },
    ],
    rows: capabilities,
    rowAttrs: (cap) => `data-searchable`,
    emptyMessage: "No capabilities yet.",
    emptyIcon: "icon-layers",
  });
}

function renderStrips(providers, alerts) {
  const pl = providers?.ok ? providers.data?.providers || [] : [];
  const al = alerts?.ok ? (alerts.data || []).filter((a) => a.status === "active" || a.status === "pending") : [];
  const providerPanel = `<article class="panel flex items-center gap-4 p-5">
    <span class="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-[#e9ecff] text-blue"><svg class="h-5 w-5 fill-none stroke-current stroke-[1.6]"><use href="#icon-play"/></svg></span>
    <span class="min-w-0 flex-1"><span class="eyebrow">LLM providers</span><span class="mt-1 block text-[.78rem] font-bold">${escape(pl.length ? pl.join(" · ") : "none")}</span></span>
    ${providers?.ok ? statusPill(`${pl.length} online`, "good") : statusPill(providers?.error || "unavailable", "warn")}
  </article>`;
  const alertPanel = al.length
    ? `<article class="panel flex items-center gap-4 p-5">
        <span class="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-[#fff0e9] text-accent"><svg class="h-5 w-5 fill-none stroke-current stroke-[1.6]"><use href="#icon-warning"/></svg></span>
        <span class="min-w-0 flex-1"><span class="eyebrow">Alerts</span><span class="mt-1 block text-[.78rem] font-bold">${al.length} signal${al.length === 1 ? "" : "s"} need review</span></span>
        <a href="#/guardrails" class="text-[.68rem] font-bold text-accent hover:underline">Inspect</a>
      </article>`
    : `<article class="panel flex items-center gap-4 p-5">
        <span class="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-[#eaf6ed] text-[#4d9665]"><svg class="h-5 w-5 fill-none stroke-current stroke-[1.6]"><use href="#icon-check"/></svg></span>
        <span class="min-w-0 flex-1"><span class="eyebrow">Alerts</span><span class="mt-1 block text-[.78rem] font-bold">All clear</span></span>
        ${statusPill("OK", "good")}
      </article>`;
  return `${providerPanel}${alertPanel}`;
}

function shell(clock) {
  const topActions = `<div class="flex shrink-0 items-center gap-2">
    <button id="action-refresh" class="quiet-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-[1.8]"><use href="#icon-refresh"/></svg><span>Refresh</span></button>
    <button id="action-new-capability" class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-[2]"><use href="#icon-plus"/></svg> New capability</button>
  </div>`;
  return `
    <header class="page-header">
      <div>
        <div class="eyebrow flex items-center gap-2">Command center <span class="h-1 w-1 rounded-full bg-accent"></span> <span id="overview-clock">${escape(clock)}</span></div>
        <h1 class="page-title mt-3 !text-[clamp(2rem,4vw,3.25rem)] !tracking-[-.075em] !leading-[.98] !font-bold">Make every capability<br class="hidden sm:block" /> production-ready.</h1>
        <p class="page-description mt-4 !max-w-xl !text-[.86rem] !leading-6">One clear view of your intelligence stack. Review what changed, promote what works, and keep every decision traceable.</p>
      </div>
      ${topActions}
    </header>
    <section id="overview-metrics" class="mt-8">${skeletonStack({ lines: 4 })}</section>
    <section class="mt-5 grid gap-5 xl:grid-cols-[minmax(0,1.55fr)_minmax(330px,.85fr)]">
      <article class="panel overflow-hidden p-5 sm:p-6">
        <div class="eyebrow">Release velocity</div>
        <h2 class="mt-2 text-[1.05rem] font-bold tracking-[-.035em]">Across the workspace</h2>
        <div class="mt-7 flex items-end gap-3"><span id="overview-capability-count" class="text-[2rem] font-bold tracking-[-.07em]">—</span><span class="mb-1 text-[.72rem] font-semibold text-[#6d8c34]">capabilities tracked <span class="text-muted">across <span id="overview-project-count">—</span> projects</span></span></div>
        <div id="overview-environment" class="mt-4 space-y-3">${skeletonStack({ lines: 1, width: "100%" })}${skeletonStack({ lines: 1, width: "100%" })}${skeletonStack({ lines: 1, width: "100%" })}</div>
        <div class="soft-divider mt-5 pt-4"><div class="flex items-center justify-between text-[.68rem]"><span class="text-muted">Last activity</span><span id="overview-last-activity" class="font-bold text-[#646761]">—</span></span></div>
      </article>
      <article class="panel-dark overflow-hidden p-5 sm:p-6">
        <div class="flex items-start justify-between"><div><div class="eyebrow !text-[#898b92]">Needs your attention</div><h2 class="mt-2 text-[1.05rem] font-bold tracking-[-.035em]">Pending releases</h2></div><span class="grid h-9 w-9 place-items-center rounded-xl bg-accent/15 text-accent"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.8]"><use href="#icon-clock"/></svg></span></div>
        <div id="overview-pending">${skeletonStack({ lines: 2 })}</div>
        <p class="mt-5 text-[.66rem] text-[#898c94]">Releases are surfaced from <span class="mono">/api/v1/capabilities/{id}/releases</span>.</p>
      </article>
    </section>
    <section class="mt-5 grid gap-5 xl:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)]">
      ${panel({ eyebrow: "Recent activity", title: "Live from the audit log", body: `<div id="overview-activity">${skeletonStack({ lines: 3 })}</div>`, className: "overflow-hidden" })}
      ${panel({ eyebrow: "Capability catalog", title: "Everything in the workspace", rightSlot: `<span class="text-[.6rem] text-muted" id="overview-capabilities-count">—</span>`, body: `<div id="overview-capabilities">${skeletonStack({ lines: 3 })}</div>`, className: "overflow-hidden" })}
    </section>
    <section class="mt-5 grid gap-5 md:grid-cols-2"><div id="overview-strip-1">${skeletonStack({ lines: 1 })}</div><div id="overview-strip-2">${skeletonStack({ lines: 1 })}</div></section>
    <footer class="flex flex-col justify-between gap-2 py-8 text-[.64rem] text-muted sm:flex-row sm:items-center"><span>Promptsheon control plane <span class="mx-1 text-[#b7b8b3]">·</span> <span id="runtime-status-footer">Loading</span></span><span class="mono">build <span id="runtime-version-footer">v0.3.0</span></span></footer>
  `;
}

export async function renderOverview(route) {
  const { loadSettings } = await import("../settings.js");
  if (!loadSettings().apiKey) {
    return renderConnectPrompt();
  }
  const initialFilter = route?.query?.action || (window.localStorage.getItem("promptsheon.auditFilter") || "all");
  const clock = new Intl.DateTimeFormat("en-US", { weekday: "long", month: "short", day: "numeric" }).format(new Date());
  const html = shell(clock);
  window.document.getElementById("view").innerHTML = html;
  queueMicrotask(async () => {
    try {
      const data = await loadOverviewData(initialFilter);
      applyRuntimePill(data.health, data.ready);
      applyOverviewData(data);
      attachOverviewActions(data);
    } catch (error) {
      console.error("Overview hydration failed:", error);
    }
  });
  return html;
}

function renderConnectPrompt() {
  const html = `<section class="panel p-8 text-center">
    <div class="mx-auto grid h-12 w-12 place-items-center rounded-2xl bg-paper text-muted"><svg class="h-6 w-6 fill-none stroke-current stroke-2"><use href="#icon-key"/></svg></div>
    <h1 class="mt-5 page-title">Connect the Promptsheon API</h1>
    <p class="mt-2 page-description mx-auto">Open <span class="font-bold text-ink">Connection</span> in the sidebar to paste an API key or set a custom base URL.</p>
    <div class="mt-5 flex justify-center gap-2">
      <button data-open-settings class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-settings"/></svg>Open Connection</button>
    </div>
  </section>`;
  const view = window.document.getElementById("view");
  if (view) view.innerHTML = html;
  return html;
}

function applyRuntimePill(health, ready) {
  const pill = window.document.getElementById("runtime-status-pill");
  if (!pill) return;
  pill.classList.remove("good", "warn", "danger", "neutral");
  const ok = ready?.ok && ready.data?.status === "ready" && health?.ok;
  pill.classList.add(ok ? "good" : "warn");
  const label = pill.querySelector("[data-runtime-label]");
  if (label) label.textContent = ok ? "Healthy" : (health?.ok ? ready.data.status : "Connecting…");
  if (health?.ok) {
    const v = window.document.getElementById("runtime-version");
    if (v) v.textContent = health.data.version || "dev";
    const u = window.document.getElementById("runtime-uptime");
    if (u) u.textContent = health.data.uptime || "—";
  }
}

function applyOverviewData(data) {
  const metrics = window.document.getElementById("overview-metrics");
  if (metrics) metrics.innerHTML = renderMetrics(data.metrics);
  const pending = window.document.getElementById("overview-pending");
  if (pending) pending.innerHTML = renderPending(data.releases, data.capabilityMap);
  const activity = window.document.getElementById("overview-activity");
  if (activity) {
    activity.innerHTML = renderAudit(data.audit, data.auditFilter);
    activity.addEventListener("click", (event) => {
      const chip = event.target.closest("[data-audit-filter]");
      if (!chip) return;
      const next = chip.dataset.auditFilter;
      if (next === data.auditFilter) return;
      window.localStorage.setItem("promptsheon.auditFilter", next);
      const action = next === "all" ? "" : `?action=${encodeURIComponent(next)}`;
      window.location.hash = `#/${action}`;
      window.location.reload();
    });
  }
  const env = window.document.getElementById("overview-environment");
  if (env) env.innerHTML = renderEnvironments(data.releases, data.capabilityMap);
  const caps = window.document.getElementById("overview-capabilities");
  if (caps) caps.innerHTML = renderCapabilities(data.capabilities, data.releases);
  const strip1 = window.document.getElementById("overview-strip-1");
  const strip2 = window.document.getElementById("overview-strip-2");
  if (strip1 && strip2) {
    const html = renderStrips(data.providers, data.alerts);
    const [s1, s2] = html.match(/<article[\s\S]*?<\/article>/g) || [];
    strip1.innerHTML = s1 || "";
    strip2.innerHTML = s2 || "";
  }
  const capsCount = window.document.getElementById("overview-capability-count");
  if (capsCount) capsCount.textContent = data.capabilities.length || "0";
  const projCount = window.document.getElementById("overview-project-count");
  if (projCount) projCount.textContent = data.projects.length || "0";
  const capsRow = window.document.getElementById("overview-capabilities-count");
  if (capsRow) capsRow.textContent = `${data.capabilities.length} capabilities`;
  const navCaps = window.document.getElementById("nav-capabilities-count");
  if (navCaps) navCaps.textContent = data.capabilities.length || "0";
  const navReleases = window.document.getElementById("nav-releases-count");
  if (navReleases) navReleases.textContent = data.releases.filter((r) => r.status === "pending").length;
  const last = (data.audit?.data || [])[0];
  const lastEl = window.document.getElementById("overview-last-activity");
  if (lastEl) lastEl.textContent = last ? formatRelative(last.timestamp) : "—";
  const clock = window.document.getElementById("overview-clock");
  if (clock) clock.textContent = new Intl.DateTimeFormat("en-US", { weekday: "long", month: "short", day: "numeric" }).format(new Date());
  const footer = window.document.getElementById("runtime-status-footer");
  if (footer) {
    if (data.health?.ok && data.ready?.ok && data.ready.data?.status === "ready") {
      footer.textContent = "Operational";
    } else if (data.metrics?.status === 401 || data.workspaces?.status === 401) {
      footer.textContent = "Needs API key";
    } else if (data.metrics?.status === 429 || data.workspaces?.status === 429) {
      footer.textContent = "Rate limited";
    } else if (!data.health?.ok) {
      footer.textContent = "Offline";
    } else {
      footer.textContent = "Live";
    }
  }
  const pendingSection = window.document.getElementById("overview-pending");
  if (pendingSection) pendingSection.addEventListener("click", (event) => {
    const button = event.target.closest("[data-open-release]");
    if (!button) return;
    const id = button.dataset.openRelease;
    if (!id) return;
    import("./release-modal.js").then(({ openReleaseModal }) => openReleaseModal(id)).catch((e) => console.error("Failed to open release modal:", e));
  });
}

function attachOverviewActions() {
  const refresh = window.document.getElementById("action-refresh");
  if (refresh) refresh.addEventListener("click", () => window.location.reload());
  const newCap = window.document.getElementById("action-new-capability");
  if (newCap) newCap.addEventListener("click", async () => {
    const mod = await import("./new-capability-modal.js");
    const root = window.document.getElementById("modal-root");
    if (root) await mod.openNewCapabilityModal(root, { onCreated: () => window.location.reload() });
  });
}
