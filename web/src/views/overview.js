import * as api from "../api.js";
import { escape, formatCompact, formatInteger, formatMoney, formatPercent, formatRelative } from "../utils.js";
import { ownerName } from "../state/owners.js";

const skeleton = (label, lines = 3) => `
  <div class="panel p-5">
    <div class="skeleton h-3 w-24"></div>
    ${Array.from({ length: lines }).map(() => '<div class="skeleton mt-4 h-4"></div>').join("")}
    <span class="sr-only">Loading ${escape(label)}…</span>
  </div>`;

const card = (eyebrow, icon, value, sub, tone = "neutral") => `
  <article class="panel p-5">
    <div class="flex items-start justify-between"><span class="eyebrow">${escape(eyebrow)}</span><span class="grid h-8 w-8 place-items-center rounded-lg bg-paper"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-${escape(icon)}"/></svg></span></div>
    <div class="mt-6 flex items-end justify-between gap-3"><span class="metric-value">${escape(value)}</span><span class="status-pill ${tone}"><span class="status-dot"></span>${escape(sub)}</span></div>
  </article>`;

const TONES = { active: "good", approved: "good", pending: "warn", superseded: "neutral", rolled_back: "danger" };

function pill(text, tone = "neutral") {
  return `<span class="status-pill ${tone}"><span class="status-dot"></span>${escape(text)}</span>`;
}

function sectionOpen(title, anchor) {
  return `<section class="panel p-5 sm:p-6"><div class="flex items-start justify-between"><div><div class="eyebrow">${escape(anchor)}</div><h2 class="mt-2 text-[1.05rem] font-bold tracking-[-.035em]">${escape(title)}</h2></div></div>`;
}

function errorPanel(message) {
  return `<div class="panel p-5 text-[.72rem] text-muted">${escape(message)}</div>`;
}

function emptyPanel(text) {
  return `<div class="rounded-xl border border-dashed border-line bg-paper p-4 text-center text-[.7rem] text-muted">${escape(text)}</div>`;
}

async function loadOverviewData() {
  const out = {
    health: null, ready: null, metrics: null, providers: null,
    alerts: null, audit: null, workspaces: null,
    projects: [], capabilities: [], releases: [], capabilityMap: new Map()
  };
  out.health = await api.getHealth();
  out.ready = await api.getReady();
  if (!out.health.ok && !out.ready.ok) return out;
  const [metrics, providers, alerts, audit, workspaces] = await Promise.all([
    api.getMetricsSummary(), api.listProviders(), api.listAlerts(),
    api.listAudit(12), api.listWorkspaces()
  ]);
  out.metrics = metrics; out.providers = providers; out.alerts = alerts;
  out.audit = audit; out.workspaces = workspaces;
  if (workspaces.ok && workspaces.data?.length) {
    const w = workspaces.data[0];
    const projects = await api.listProjects(w.id);
    if (projects.ok) {
      out.projects = projects.data || [];
      const capLists = await Promise.all(out.projects.map((p) => api.listCapabilities(p.id)));
      out.capabilities = capLists.filter((r) => r.ok).flatMap((r) => r.data || []);
      const relLists = await Promise.all(out.capabilities.map((c) => api.listReleases(c.id)));
      out.releases = relLists.filter((r) => r.ok).flatMap((r) => r.data || []);
      out.capabilityMap = new Map(out.capabilities.map((c) => [c.id, c]));
    }
  }
  return out;
}

function renderMetrics(metrics) {
  if (!metrics || !metrics.ok || !metrics.data) {
    const msg = metrics?.status === 401 ? "Live counters unlock once an API key is set."
      : metrics?.status === 429 ? "Daemon is rate limiting requests. Slowing down."
      : metrics?.error ? `Live counters unavailable (${escape(metrics.error)}).`
      : "Live counters unavailable while the daemon warms up.";
    return errorPanel(msg);
  }
  const api = metrics.data.api_metrics || {};
  const llm = metrics.data.llm_metrics || {};
  const review = metrics.data.review_metrics || {};
  const guardrail = metrics.data.guardrail_metrics || {};
  const errorRate = api.error_rate || 0;
  const reqBadge = errorRate > 0 ? "warn" : "good";
  const reqLabel = errorRate > 0 ? "Attention" : "Healthy";
  const reviewBadge = (review.pending_count || 0) > 0 ? "warn" : "good";
  return `
    <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      ${card("API requests", "pulse", formatCompact(api.total_requests), reqLabel, reqBadge)}
      ${card("LLM spend", "rocket", formatMoney(llm.total_cost_usd), `${formatCompact(llm.total_calls)} calls`, "neutral")}
      ${card("Reviews", "scroll", formatInteger(review.total_reviews), `${formatInteger(review.pending_count)} pending`, reviewBadge)}
      ${card("Guardrails", "shield", formatInteger((guardrail.passes || 0) + (guardrail.blocks || 0)), `${formatInteger(guardrail.blocks || 0)} blocks`, (guardrail.blocks || 0) > 0 ? "warn" : "good")}
    </div>`;
}

function renderEnvironments(releases, capabilitiesById) {
  const envs = ["prod", "staging", "dev"];
  return envs.map((env) => {
    const list = releases.filter((r) => r.environment === env);
    const active = list.find((r) => r.status === "active");
    const pending = list.filter((r) => r.status === "pending").length;
    const cap = active ? capabilitiesById.get(active.capability_id) : null;
    const label = active ? `${cap?.name || "capability"} v${active.capability_version}` : pending > 0 ? `${pending} pending review` : "no active release";
    const tone = env === "prod" ? "good" : env === "staging" ? "warn" : "neutral";
    const status = active ? "Active" : pending > 0 ? "Pending" : "Idle";
    return `<div class="flex items-center gap-3 rounded-xl bg-paper p-3">
      <span class="grid h-9 w-9 shrink-0 place-items-center rounded-[10px] bg-white text-[#5c5e63]"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-layers"/></svg></span>
      <span class="min-w-0 flex-1"><span class="flex items-center gap-2 text-[.75rem] font-bold">${escape(env)} <span class="mono text-[.58rem] font-medium text-muted">${escape(list.length)} total</span></span><span class="mt-1 block text-[.64rem] text-muted">${escape(label)}</span></span>
      ${pill(status, tone)}
    </div>`;
  }).join("");
}

function renderPending(releases, capabilitiesById) {
  const pending = releases.filter((r) => r.status === "pending");
  if (!pending.length) {
    return `<div class="mt-5 rounded-xl border border-dashed border-white/8 bg-white/[.04] p-5 text-center"><p class="text-[.78rem] font-bold text-white">Nothing waiting on you</p><p class="mt-1 text-[.66rem] text-[#9a9da4]">All releases are either approved or shipped.</p></div>`;
  }
  return `<div class="mt-5 space-y-2">${pending.slice(0, 6).map((r) => {
    const cap = capabilitiesById.get(r.capability_id);
    const name = cap?.name || "Unknown capability";
    const initials = (name || "?").split(/\s+/).filter(Boolean).slice(0,2).map((p) => (p[0]||"").toUpperCase()).join("");
    return `<button type="button" data-open-release="${escape(r.id)}" class="flex w-full items-center gap-3 rounded-xl border border-white/8 bg-white/[.045] p-3 text-left transition hover:bg-white/[.09]">
      <span class="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-[#30323a] text-[.65rem] font-bold text-[#c5f06e]">${escape(initials)}</span>
      <span class="min-w-0 flex-1"><span class="block truncate text-[.75rem] font-bold">${escape(name)}</span><span class="mt-1 block text-[.65rem] text-[#898c94]">v${escape(r.capability_version)} · ${escape(r.environment)} · created ${escape(formatRelative(r.created_at))}</span></span>
      ${pill("Review", "warn")}
      <svg class="h-3.5 w-3.5 shrink-0 fill-none stroke-[#777a83] stroke-2"><use href="#icon-arrow-right"/></svg>
    </button>`;
  }).join("")}</div>`;
}

function renderAudit(audit) {
  if (!audit || !audit.ok || !audit.data) {
    if (audit?.status === 429) return errorPanel("Audit log rate-limited. Retrying automatically.");
    return errorPanel("Live activity unavailable" + (audit?.error ? ` (${escape(audit.error)})` : "") + ".");
  }
  const rows = (audit.data || []).slice(0, 8);
  if (!rows.length) return emptyPanel("No activity yet. Create a capability or release to populate the audit trail.");
  const tone = (action) => ({ delete: "danger", update: "warn", create: "good", activate: "good", rollback: "danger" }[action]) || "neutral";
  return `<div class="mt-5 space-y-3">${rows.map((entry) => {
    const details = entry.details || {};
    const target = (entry.resource || "").split(":").pop();
    const label = details.name || target || entry.resource;
    const subject = (entry.user_id || "api") === "api" ? "System" : entry.user_id;
    return `<div class="flex gap-3">
      <span class="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-paper text-[#5c5e63]"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-pulse"/></svg></span>
      <div class="min-w-0 flex-1"><p class="text-[.74rem] leading-5"><span class="font-bold">${escape(subject)}</span> · <span class="font-semibold">${escape(entry.action)}</span> · <span class="font-bold">${escape(label)}</span></p><p class="mt-0.5 text-[.63rem] text-muted">${escape(formatRelative(entry.timestamp))} <span class="mx-1 text-[#c1c2bd]">·</span> <span class="mono">${escape(entry.resource)}</span></p></div>
      ${pill(entry.action, tone(entry.action))}
    </div>`;
  }).join("")}</div>`;
}

function renderCapabilities(capabilities, releases) {
  if (!capabilities.length) {
    return `<div class="px-5 py-10 text-center"><p class="text-[.78rem] font-bold">No capabilities yet</p><p class="mt-1 text-[.66rem] text-muted">Use the <span class="font-semibold text-ink">New capability</span> button above to add the first one.</p></div>`;
  }
  const byCap = new Map();
  for (const r of releases) {
    if (!byCap.has(r.capability_id)) byCap.set(r.capability_id, []);
    byCap.get(r.capability_id).push(r);
  }
  return capabilities.map((cap) => {
    const rels = byCap.get(cap.id) || [];
    const release = rels.find((r) => r.status === "active") || rels.find((r) => r.status === "pending") || rels[0];
    const version = rels.reduce((m, r) => Math.max(m, r.capability_version || 0), 0) || 1;
    const pillLabel = !release ? "Draft" : ({ active: "Production", approved: "Approved", pending: "Pending", superseded: "Superseded", rolled_back: "Rolled back" })[release.status] || "Draft";
    const tone = release && release.environment === "prod" ? "good" : release && release.environment === "staging" ? "warn" : "neutral";
    return `<a href="#/capabilities/${escape(cap.id)}" data-searchable class="data-row grid gap-3 py-4 md:grid-cols-[minmax(220px,1.7fr)_110px_120px_130px_120px_32px] md:items-center md:gap-4">
      <div class="flex min-w-0 items-center gap-3">
        <span class="grid h-9 w-9 shrink-0 place-items-center rounded-[10px] bg-[#e9ecff] text-blue"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-spark"/></svg></span>
        <span class="min-w-0"><span class="block truncate text-[.78rem] font-bold">${escape(cap.name)}</span><span class="mt-1 block truncate text-[.64rem] text-muted">${escape(cap.description || (cap.tags && cap.tags.join(" · ")) || "No description")}</span></span>
      </div>
      <span class="mono text-[.68rem] text-[#62656a]">v${escape(version)}</span>
      <span>${pill(pillLabel, tone)}</span>
      <span class="text-[.74rem] font-bold">— <span class="block text-[.62rem] font-medium text-muted">no evals yet</span></span>
      <span class="text-[.72rem] text-[#686b70]">${escape(ownerName(cap.owner))}</span>
      <span class="icon-button !h-7 !w-7 !border-0 !bg-transparent"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.8]"><use href="#icon-arrow-right"/></svg></span>
    </a>`;
  }).join("");
}

function renderStrips(providers, alerts) {
  const pl = providers?.ok ? providers.data?.providers || [] : [];
  const al = alerts?.ok ? (alerts.data || []).filter((a) => a.status === "active" || a.status === "pending") : [];
  const providerPanel = `<article class="panel flex items-center gap-4 p-5">
    <span class="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-[#e9ecff] text-blue"><svg class="h-5 w-5 fill-none stroke-current stroke-[1.6]"><use href="#icon-play"/></svg></span>
    <span class="min-w-0 flex-1"><span class="eyebrow">LLM providers</span><span class="mt-1 block text-[.78rem] font-bold">${escape(pl.length ? pl.join(" · ") : "none")}</span></span>
    ${providers?.ok ? pill(`${pl.length} online`, "good") : pill(providers?.error || "unavailable", "warn")}
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
        ${pill("OK", "good")}
      </article>`;
  return `${providerPanel}${alertPanel}`;
}

export async function renderOverview(route) {
  const body = window.document.getElementById("view-body");
  if (!body) return "";
  body.innerHTML = `
    <section class="flex flex-col justify-between gap-5 md:flex-row md:items-end">
      <div>
        <div class="eyebrow flex items-center gap-2">Command center <span class="h-1 w-1 rounded-full bg-accent"></span> <span id="overview-clock">—</span></div>
        <h1 class="mt-3 max-w-2xl text-[clamp(2rem,4vw,3.25rem)] font-bold leading-[.98] tracking-[-.075em]">Make every capability<br class="hidden sm:block" /> production-ready.</h1>
        <p class="mt-4 max-w-xl text-[.86rem] leading-6 text-muted">One clear view of your intelligence stack. Review what changed, promote what works, and keep every decision traceable.</p>
      </div>
      <div class="flex shrink-0 items-center gap-2">
        <button id="action-refresh" class="quiet-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-[1.8]"><use href="#icon-refresh"/></svg><span>Refresh</span></button>
        <button id="action-new-capability" class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-[2]"><use href="#icon-plus"/></svg> New capability</button>
      </div>
    </section>
    <section id="overview-metrics" class="mt-8">${skeleton("metrics", 4)}</section>
    <section class="mt-5 grid gap-5 xl:grid-cols-[minmax(0,1.55fr)_minmax(330px,.85fr)]">
      <article class="panel overflow-hidden p-5 sm:p-6">${sectionOpen("Across the workspace", "Release velocity")}<div class="mt-7 flex items-end gap-3"><span id="overview-capability-count" class="text-[2rem] font-bold tracking-[-.07em]">—</span><span class="mb-1 text-[.72rem] font-semibold text-[#6d8c34]">capabilities tracked <span class="text-muted">across <span id="overview-project-count">—</span> projects</span></span></div><div id="overview-environment" class="mt-4 space-y-3">${skeleton("environments", 1)}${skeleton("environments", 1)}${skeleton("environments", 1)}</div><div class="soft-divider mt-5 pt-4"><div class="flex items-center justify-between text-[.68rem]"><span class="text-muted">Last activity</span><span id="overview-last-activity" class="font-bold text-[#646761]">—</span></span></div></div></article>
      <article class="panel-dark overflow-hidden p-5 sm:p-6">
        <div class="flex items-start justify-between"><div><div class="eyebrow !text-[#898b92]">Needs your attention</div><h2 class="mt-2 text-[1.05rem] font-bold tracking-[-.035em]">Pending releases</h2></div><span class="grid h-9 w-9 place-items-center rounded-xl bg-accent/15 text-accent"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.8]"><use href="#icon-clock"/></svg></span></div>
        <div id="overview-pending">${skeleton("pending", 2)}</div>
        <p class="mt-5 text-[.66rem] text-[#898c94]">Releases are surfaced from <span class="mono">/api/v1/capabilities/{id}/releases</span>.</p>
      </article>
    </section>
    <section class="mt-5 grid gap-5 xl:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)]">
      <article class="panel overflow-hidden p-5 sm:p-6">
        <div class="flex items-start justify-between"><div><div class="eyebrow">Recent activity</div><h2 class="mt-2 text-[1.05rem] font-bold tracking-[-.035em]">Live from the audit log</h2></div></div>
        <div id="overview-activity">${skeleton("audit", 3)}</div>
      </article>
      <article class="panel overflow-hidden p-5 sm:p-6">
        <div class="flex items-start justify-between"><div><div class="eyebrow">Capability catalog</div><h2 class="mt-2 text-[1.05rem] font-bold tracking-[-.035em]">Everything in the workspace</h2></div><span class="text-[.6rem] text-muted" id="overview-capabilities-count">—</span></div>
        <div class="hidden grid-cols-[minmax(220px,1.7fr)_110px_120px_130px_120px_32px] gap-4 border-b border-line/70 bg-paper/70 px-1 pb-3 pt-4 text-[.62rem] font-bold uppercase tracking-[.12em] text-muted md:grid"><span>Capability</span><span>Version</span><span>Status</span><span>Reliability</span><span>Owner</span><span></span></div>
        <div id="overview-capabilities">${skeleton("capabilities", 3)}</div>
      </article>
    </section>
    <section class="mt-5 grid gap-5 md:grid-cols-2"><div id="overview-strip-1">${skeleton("providers", 1)}</div><div id="overview-strip-2">${skeleton("alerts", 1)}</div></section>
    <footer class="flex flex-col justify-between gap-2 py-8 text-[.64rem] text-muted sm:flex-row sm:items-center"><span>Promptsheon control plane <span class="mx-1 text-[#b7b8b3]">·</span> <span id="runtime-status-footer">Loading</span></span><span class="mono">build <span id="runtime-version-footer">v0.3.0</span></span></footer>
  `;

  const data = await loadOverviewData();
  applyRuntimePill(data.health, data.ready);
  applyOverviewData(data);
  return "";
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
  if (activity) activity.innerHTML = renderAudit(data.audit);
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
  attachOverviewActions(data);
}

function attachOverviewActions(data) {
  const refresh = window.document.getElementById("action-refresh");
  refresh?.addEventListener("click", () => window.location.reload());
  const newCap = window.document.getElementById("action-new-capability");
  newCap?.addEventListener("click", () => {
    window.dispatchEvent(new CustomEvent("promptsheon:new-capability", { detail: { projects: data.projects } }));
  });
}
