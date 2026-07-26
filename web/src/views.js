const escape = (value) => String(value ?? "").replace(/[&<>"']/g, (ch) => ({
  "&": "&amp;",
  "<": "&lt;",
  ">": "&gt;",
  '"': "&quot;",
  "'": "&#39;"
})[ch]);

const formatCompact = (value) => {
  if (typeof value !== "number" || Number.isNaN(value)) return "0";
  return new Intl.NumberFormat("en-US", { notation: "compact", maximumFractionDigits: 1 }).format(value);
};

const formatInteger = (value) => {
  if (typeof value !== "number" || Number.isNaN(value)) return "0";
  return new Intl.NumberFormat("en-US").format(value);
};

const formatMoney = (value) => {
  if (typeof value !== "number" || Number.isNaN(value)) return "$0";
  return new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 0 }).format(value);
};

const formatPercent = (value) => {
  if (typeof value !== "number" || Number.isNaN(value)) return "—";
  return `${value.toFixed(2)}%`;
};

const formatRelative = (iso) => {
  if (!iso) return "—";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "—";
  const diff = (Date.now() - date.getTime()) / 1000;
  if (diff < 60) return `${Math.round(diff)}s ago`;
  if (diff < 3600) return `${Math.round(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.round(diff / 3600)}h ago`;
  if (diff < 604800) return `${Math.round(diff / 86400)}d ago`;
  return new Intl.DateTimeFormat("en-US", { month: "short", day: "numeric" }).format(date);
};

const initials = (name) => {
  if (!name) return "—";
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() || "")
    .join("") || name.slice(0, 2).toUpperCase();
};

const statusPill = (label, tone = "neutral") => `<span class="status-pill ${tone}"><span class="status-dot"></span>${escape(label)}</span>`;

export function renderMetrics(metrics) {
  if (!metrics || !metrics.ok || !metrics.data) {
    const tone = metrics?.status === 429 ? "warn" : metrics?.status === 401 ? "warn" : "neutral";
    const reason = metrics?.status === 401
      ? "Live counters unlock once an API key is set."
      : metrics?.status === 429
      ? "Daemon is rate limiting requests. Slowing down."
      : metrics?.error
      ? `Live counters unavailable (${escape(metrics.error)}).`
      : "Live counters unavailable while the daemon warms up.";
    return `<div class="panel p-5 text-[.72rem] text-muted">${reason}</div>`;
  }
  const api = metrics.data.api_metrics || {};
  const llm = metrics.data.llm_metrics || {};
  const review = metrics.data.review_metrics || {};
  const guardrail = metrics.data.guardrail_metrics || {};
  return `
    <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4" aria-label="Key metrics">
      <article class="panel group p-5 transition hover:-translate-y-0.5">
        <div class="flex items-start justify-between"><span class="eyebrow">API requests</span><span class="grid h-8 w-8 place-items-center rounded-lg bg-[#eef3dc] text-[#719335]"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-pulse"/></svg></span></div>
        <div class="mt-6 flex items-end justify-between gap-3"><span class="metric-value">${formatCompact(api.total_requests)}</span>${statusPill(formatPercent(api.error_rate || 0).startsWith("0") ? "healthy" : "attention", api.error_rate > 0 ? "warn" : "good")}</div>
        <div class="mt-4 text-[.68rem] text-muted">p95 <span class="font-semibold text-[#55585e]">${escape(Math.round(api.p95_latency_ms || 0))}ms</span> · p99 <span class="font-semibold text-[#55585e]">${escape(Math.round(api.p99_latency_ms || 0))}ms</span></div>
      </article>
      <article class="panel group p-5 transition hover:-translate-y-0.5">
        <div class="flex items-start justify-between"><span class="eyebrow">LLM spend (cumulative)</span><span class="grid h-8 w-8 place-items-center rounded-lg bg-[#fff0e9] text-accent"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-rocket"/></svg></span></div>
        <div class="mt-6 flex items-end justify-between gap-3"><span class="metric-value">${escape(formatMoney(llm.total_cost_usd))}</span>${statusPill(`${formatCompact(llm.total_calls)} calls`, "neutral")}</div>
        <div class="mt-4 text-[.68rem] text-muted">${formatInteger(llm.total_tokens)} tokens · avg <span class="font-semibold text-[#55585e]">${escape(Math.round(llm.avg_latency_ms || 0))}ms</span></div>
      </article>
      <article class="panel group p-5 transition hover:-translate-y-0.5">
        <div class="flex items-start justify-between"><span class="eyebrow">Reviews</span><span class="grid h-8 w-8 place-items-center rounded-lg bg-[#e9ecff] text-blue"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-scroll"/></svg></span></div>
        <div class="mt-6 flex items-end justify-between gap-3"><span class="metric-value">${formatInteger(review.total_reviews || 0)}</span>${statusPill(`${formatInteger(review.pending_count || 0)} pending`, (review.pending_count || 0) > 0 ? "warn" : "good")}</div>
        <div class="mt-4 text-[.68rem] text-muted">approval rate <span class="font-semibold text-[#55585e]">${escape(formatPercent(review.approval_rate || 0))}</span> · ${formatInteger(review.rejected_count || 0)} rejected</div>
      </article>
      <article class="panel group p-5 transition hover:-translate-y-0.5">
        <div class="flex items-start justify-between"><span class="eyebrow">Guardrails</span><span class="grid h-8 w-8 place-items-center rounded-lg bg-[#f0eaff] text-violet"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-shield"/></svg></span></div>
        <div class="mt-6 flex items-end justify-between gap-3"><span class="metric-value">${formatInteger((guardrail.passes || 0) + (guardrail.blocks || 0))}</span>${statusPill(`${formatInteger(guardrail.blocks || 0)} blocks`, (guardrail.blocks || 0) > 0 ? "warn" : "good")}</div>
        <div class="mt-4 text-[.68rem] text-muted">${formatInteger(guardrail.violations || 0)} violations · ${formatInteger(guardrail.passes || 0)} passes</div>
      </article>
    </div>`;
}

export function renderEnvironmentSummary(releases, capabilitiesById) {
  if (!releases || !releases.data) {
    return `<div class="mt-6 space-y-3 text-[.72rem] text-muted">Live environment data unavailable.</div>`;
  }
  const envs = ["prod", "staging", "dev"];
  const tone = (env) => env === "prod" ? "good" : env === "staging" ? "warn" : "neutral";
  const all = releases.data;
  return envs.map((env) => {
    const list = all.filter((r) => r.environment === env);
    const active = list.find((r) => r.status === "active");
    const pending = list.filter((r) => r.status === "pending").length;
    const capNames = list.map((r) => capabilitiesById?.get(r.capability_id)?.name).filter(Boolean);
    return `<div class="flex items-center gap-3 rounded-xl bg-paper p-3">
      <span class="grid h-9 w-9 shrink-0 place-items-center rounded-[10px] bg-white text-[#5c5e63]"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-layers"/></svg></span>
      <span class="min-w-0 flex-1"><span class="flex items-center gap-2 text-[.75rem] font-bold">${escape(env)} <span class="mono text-[.58rem] font-medium text-muted">${escape(list.length)} total</span></span><span class="mt-1 block text-[.64rem] text-muted">${active ? `active: ${escape(capNames[0] || 'capability')} v${escape(active.capability_version)}` : pending > 0 ? `${pending} pending review` : "no active release"}</span></span>
      ${statusPill(active ? "Active" : pending > 0 ? "Pending" : "Idle", tone(env))}
    </div>`;
  }).join("");
}

export function renderPendingReleases(releases, capabilitiesById) {
  if (!releases || !releases.data) {
    return `<div class="mt-5 text-[.72rem] text-muted">Live release data unavailable.</div>`;
  }
  const pending = releases.data.filter((r) => r.status === "pending");
  if (pending.length === 0) {
    return `<div class="mt-5 rounded-xl border border-dashed border-white/8 bg-white/[.04] p-5 text-center"><p class="text-[.78rem] font-bold text-white">Nothing waiting on you</p><p class="mt-1 text-[.66rem] text-[#9a9da4]">All releases are either approved or shipped.</p></div>`;
  }
  return `<div class="mt-5 space-y-2">${pending.slice(0, 6).map((r) => {
    const cap = capabilitiesById.get(r.capability_id);
    const name = cap?.name || "Unknown capability";
    return `<button type="button" data-open-release="${escape(r.id)}" class="flex w-full items-center gap-3 rounded-xl border border-white/8 bg-white/[.045] p-3 text-left transition hover:bg-white/[.09]">
      <span class="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-[#30323a] text-[.65rem] font-bold text-[#c5f06e]">${escape(initials(name))}</span>
      <span class="min-w-0 flex-1"><span class="block truncate text-[.75rem] font-bold">${escape(name)}</span><span class="mt-1 block text-[.65rem] text-[#898c94]">v${escape(r.capability_version)} · ${escape(r.environment)} · created ${escape(formatRelative(r.created_at))}</span></span>
      <span class="status-pill warn !bg-[#3a3020] !text-[#f0bd5c]">Review</span>
      <svg class="h-3.5 w-3.5 shrink-0 fill-none stroke-[#777a83] stroke-2"><use href="#icon-arrow-right"/></svg>
    </button>`;
  }).join("")}</div>`;
}

export function renderAuditFeed(audit) {
  if (!audit || !audit.ok || !audit.data) {
    if (audit?.status === 429) {
      return `<div class="mt-5 rounded-xl border border-amber-200 bg-amber-50 p-4 text-[.7rem] text-amber-900">Audit log rate-limited. Retrying automatically.</div>`;
    }
    return `<div class="mt-5 text-[.72rem] text-muted">Live activity unavailable${audit?.error ? ` (${escape(audit.error)})` : ""}.</div>`;
  }
  const rows = (audit.data || []).slice(0, 8);
  if (rows.length === 0) {
    return `<div class="mt-5 rounded-xl border border-dashed border-line bg-paper p-5 text-center"><p class="text-[.78rem] font-bold">No activity yet</p><p class="mt-1 text-[.66rem] text-muted">Create a capability or release to populate the audit trail.</p></div>`;
  }
  const toneFor = (action) => {
    if (action === "delete") return "danger";
    if (action === "update") return "warn";
    if (action === "create") return "good";
    if (action === "activate") return "good";
    if (action === "rollback") return "danger";
    return "neutral";
  };
  return `<div class="mt-5 space-y-3">${rows.map((entry) => {
    const details = entry.details || {};
    const target = (entry.resource || "").split(":").pop();
    const label = details.name || target || entry.resource;
    const subject = (entry.user_id || "api") === "api" ? "System" : entry.user_id;
    return `<div class="flex gap-3">
      <span class="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-paper text-[#5c5e63]"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-pulse"/></svg></span>
      <div class="min-w-0 flex-1"><p class="text-[.74rem] leading-5"><span class="font-bold">${escape(subject)}</span> · <span class="font-semibold">${escape(entry.action)}</span> · <span class="font-bold">${escape(label)}</span></p><p class="mt-0.5 text-[.63rem] text-muted">${escape(formatRelative(entry.timestamp))} <span class="mx-1 text-[#c1c2bd]">·</span> <span class="mono">${escape(entry.resource)}</span></p></div>
      <span class="status-pill ${toneFor(entry.action)} !px-2 !py-1">${escape(entry.action)}</span>
    </div>`;
  }).join("")}</div>`;
}

export function renderCapabilityTable(capabilities, capabilitiesById) {
  if (!capabilities || !capabilities.data) {
    return `<div class="px-5 py-6 text-[.72rem] text-muted sm:px-6">Live catalog unavailable.</div>`;
  }
  const rows = (capabilities.data || []).slice(0, 10);
  if (rows.length === 0) {
    return `<div class="px-5 py-10 text-center sm:px-6"><p class="text-[.78rem] font-bold">No capabilities yet</p><p class="mt-1 text-[.66rem] text-muted">Use the <span class="font-semibold text-ink">New capability</span> button above to add the first one.</p></div>`;
  }
  return rows.map((cap) => {
    const status = cap._release ? cap._release.status : null;
    const env = cap._release ? cap._release.environment : null;
    const tone = env === "prod" ? "good" : env === "staging" ? "warn" : "neutral";
    const pillLabel = status === "active" ? "Production"
      : status === "approved" ? "Approved"
      : status === "pending" ? "Pending"
      : status === "superseded" ? "Superseded"
      : status === "rolled_back" ? "Rolled back"
      : "Draft";
    return `<div data-searchable class="data-row grid gap-3 py-4 md:grid-cols-[minmax(220px,1.7fr)_110px_120px_130px_120px_32px] md:items-center md:gap-4">
      <div class="flex min-w-0 items-center gap-3">
        <span class="grid h-9 w-9 shrink-0 place-items-center rounded-[10px] bg-[#e9ecff] text-blue"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-spark"/></svg></span>
        <span class="min-w-0"><span class="block truncate text-[.78rem] font-bold">${escape(cap.name)}</span><span class="mt-1 block truncate text-[.64rem] text-muted">${escape(cap.description || (cap.tags && cap.tags.join(" · ")) || "No description")}</span></span>
      </div>
      <span class="mono text-[.68rem] text-[#62656a]">v${escape(cap._latestVersion || 1)}</span>
      <span>${statusPill(pillLabel, tone)}</span>
      <span class="text-[.74rem] font-bold">— <span class="block text-[.62rem] font-medium text-muted">no evals yet</span></span>
      <span class="text-[.72rem] text-[#686b70]">${escape(cap.owner || "—")}</span>
      <button class="icon-button !h-7 !w-7 !border-0 !bg-transparent" aria-label="Open capability"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.8]"><use href="#icon-arrow-right"/></svg></button>
    </div>`;
  }).join("");
}

export function renderProviderStrip(providers, metrics) {
  if (!providers || !providers.ok || !providers.data) {
    return `<span class="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-[#e9ecff] text-blue"><svg class="h-5 w-5 fill-none stroke-current stroke-[1.6]"><use href="#icon-play"/></svg></span><span class="min-w-0 flex-1"><span class="eyebrow">LLM providers</span><span class="mt-1 block text-[.78rem] font-bold">${providers?.status === 429 ? "Rate limited — retrying" : "Registry unavailable"}</span></span><span class="status-pill warn">${escape(providers?.error || "no data")}</span>`;
  }
  const list = providers.data.providers || providers.data || [];
  const calls = metrics?.data?.llm_metrics?.total_calls || 0;
  return `<span class="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-[#e9ecff] text-blue"><svg class="h-5 w-5 fill-none stroke-current stroke-[1.6]"><use href="#icon-play"/></svg></span><span class="min-w-0 flex-1"><span class="eyebrow">LLM providers</span><span class="mt-1 block text-[.78rem] font-bold">${escape(list.length ? list.join(" · ") : "none")}</span></span><span class="status-pill good"><span class="status-dot"></span>${escape(list.length)} online</span>`;
}

export function renderAlertsStrip(alerts) {
  if (!alerts || !alerts.ok || !alerts.data) {
    return `<span class="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-[#fff0e9] text-accent"><svg class="h-5 w-5 fill-none stroke-current stroke-[1.6]"><use href="#icon-warning"/></svg></span><span class="min-w-0 flex-1"><span class="eyebrow">Alerts</span><span class="mt-1 block text-[.78rem] font-bold">Signal feed unavailable</span></span><button class="text-[.68rem] font-bold text-accent hover:underline" data-action="refresh-alerts">Retry</button>`;
  }
  const list = alerts.data.filter((a) => a.status === "active" || a.status === "pending");
  if (list.length === 0) {
    return `<span class="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-[#eaf6ed] text-[#4d9665]"><svg class="h-5 w-5 fill-none stroke-current stroke-[1.6]"><use href="#icon-check"/></svg></span><span class="min-w-0 flex-1"><span class="eyebrow">Alerts</span><span class="mt-1 block text-[.78rem] font-bold">All clear</span></span><span class="status-pill good"><svg class="h-3 w-3 fill-none stroke-current stroke-2"><use href="#icon-check"/></svg>OK</span>`;
  }
  return `<span class="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-[#fff0e9] text-accent"><svg class="h-5 w-5 fill-none stroke-current stroke-[1.6]"><use href="#icon-warning"/></svg></span><span class="min-w-0 flex-1"><span class="eyebrow">Alerts</span><span class="mt-1 block text-[.78rem] font-bold">${escape(list.length)} signal${list.length === 1 ? "" : "s"} need review</span></span><button class="text-[.68rem] font-bold text-accent hover:underline">Inspect</button>`;
}

export function renderConnectBanner(status, message, state) {
  if (status === "ready") return "";
  if (status === "needs-key") {
    return `<div class="panel-dark mb-5 flex flex-wrap items-center justify-between gap-3 p-4 text-[.78rem]"><div class="flex items-center gap-3"><span class="grid h-8 w-8 place-items-center rounded-lg bg-accent/15 text-accent"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-warning"/></svg></span><div><p class="font-bold text-white">API key required</p><p class="text-[.66rem] text-[#a8acb4]">The daemon returned 401. Open Settings and paste a <span class="mono">ps_…</span> key, or run with <span class="mono">PROMPTSHEON_AUTH=false</span> for local dev.</p></div></div><button data-open-settings class="primary-button">Connect</button></div>`;
  }
  if (status === "loading") {
    return `<div class="panel mb-5 flex flex-wrap items-center justify-between gap-3 p-4 text-[.78rem]"><div class="flex items-center gap-3"><span class="status-dot !bg-amber-400 animate-pulse"></span><p class="font-bold">${escape(message || "Connecting…")}</p></div></div>`;
  }
  if (status === "rate-limited") {
    return `<div class="panel mb-5 flex flex-wrap items-center justify-between gap-3 border-amber-200 bg-amber-50 p-4 text-[.78rem]"><div class="flex items-center gap-3"><span class="grid h-8 w-8 place-items-center rounded-lg bg-amber-100 text-amber-700"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-clock"/></svg></span><div><p class="font-bold text-amber-900">Rate limited</p><p class="text-[.66rem] text-amber-800">${escape(message || "Slowing down and retrying.")}</p></div></div><button data-action="refresh" id="action-refresh-banner" class="quiet-button">Retry now</button></div>`;
  }
  if (status === "offline" || status === "error") {
    return `<div class="panel mb-5 flex flex-wrap items-center justify-between gap-3 border-rose-200 bg-rose-50 p-4 text-[.78rem]"><div class="flex items-center gap-3"><span class="grid h-8 w-8 place-items-center rounded-lg bg-rose-100 text-rose-600"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-warning"/></svg></span><div><p class="font-bold text-rose-900">Cannot reach the Promptsheon API</p><p class="text-[.66rem] text-rose-800">${escape(message || "Network error — start the daemon and click retry.")}</p></div></div><div class="flex gap-2"><button data-action="refresh" id="action-refresh-banner" class="quiet-button">Retry</button><button data-open-settings class="quiet-button">Settings</button></div></div>`;
  }
  return "";
}

export function renderRuntimePill(ready, health) {
  const operational = ready?.ok && ready.data?.status === "ready" && health?.ok;
  const degraded = ready?.ok && (ready.data?.status === "degraded" || ready.data?.status === "not_ready");
  let tone = "warn";
  let label = "Connecting…";
  if (operational) { tone = "good"; label = "Healthy"; }
  else if (degraded) { tone = "warn"; label = "Degraded"; }
  else if (health?.status === 0) { tone = "neutral"; label = "Offline"; }
  else if (health?.status === 401) { tone = "warn"; label = "Needs key"; }
  return { tone, label };
}

export function renderFirstRunModal({ error = null, daemonsRequireKey = false } = {}) {
  return `<div class="modal-backdrop" role="presentation">
    <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="firstrun-title">
      <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
        <div><div class="eyebrow">First run</div><h2 id="firstrun-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">Connect the Promptsheon daemon</h2><p class="mt-1 text-[.7rem] text-muted">Bootstrap a fresh admin key, paste one you already have, or point at a remote daemon.</p></div>
        <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
      </div>
      <div class="space-y-5 px-5 py-5 sm:px-6">
        <div class="rounded-xl border border-line bg-paper/60 p-4 text-[.74rem]"><p class="font-bold">Quick reference</p><p class="mt-1 text-muted">Bootstrap (one-shot, runs locally with auth off or with <span class="mono">PROMPTSHEON_BOOTSTRAP_TOKEN</span>):</p><pre class="mt-2 overflow-x-auto rounded-lg bg-ink px-3 py-2 font-mono text-[.7rem] text-lime">curl -X POST http://localhost:8080/api/v1/setup -d '{}' -H 'content-type: application/json'</pre><p class="mt-2 text-muted">Copy the <span class="mono">key</span> field and paste it below, or click <span class="font-bold">Bootstrap now</span> to do it from here.</p></div>
        <form id="settings-form" class="space-y-4">
          <div><label class="eyebrow mb-2 block" for="settings-base">API base URL</label><input id="settings-base" name="apiBase" class="field mono" data-autofocus placeholder="leave blank for /api via Vite proxy" /></div>
          <div><label class="eyebrow mb-2 block" for="settings-key">API key (ps_…)</label><input id="settings-key" name="apiKey" class="field mono" placeholder="leave blank for auth-off dev mode" /></div>
          <p id="firstrun-error" class="${error ? "" : "hidden"} rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800">${escape(error || "")}</p>
          <div class="flex flex-col gap-2 pt-1 sm:flex-row sm:justify-end">
            <button type="button" class="quiet-button" data-bootstrap-now ${daemonsRequireKey ? "" : ""}>Bootstrap now</button>
            <button type="submit" class="primary-button">Save and reload</button>
          </div>
        </form>
      </div>
    </section>
  </div>`;
}

export function renderSettingsModal(current) {
  return `<div class="modal-backdrop" role="presentation">
    <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="settings-title">
      <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
        <div><div class="eyebrow">Connection</div><h2 id="settings-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">Promptsheon API</h2><p class="mt-1 text-[.7rem] text-muted">Override the API URL or paste a key. Stored only in this browser.</p></div>
        <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
      </div>
      <form id="settings-form" class="space-y-5 px-5 py-5 sm:px-6">
        <div><label class="eyebrow mb-2 block" for="settings-base">API base URL</label><input id="settings-base" name="apiBase" class="field mono" data-autofocus value="${escape(current.apiBase || "")}" placeholder="leave blank for /api via Vite proxy" /></div>
        <div><label class="eyebrow mb-2 block" for="settings-key">API key</label><input id="settings-key" name="apiKey" class="field mono" value="${escape(current.apiKey || "")}" placeholder="ps_… (leave blank if PROMPTSHEON_AUTH=false)" /></div>
        <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
          <button type="button" class="quiet-button" data-clear-settings>Clear</button>
          <button type="submit" class="primary-button">Save and reload</button>
        </div>
      </form>
    </section>
  </div>`;
}

export function renderNewCapabilityModal(projects) {
  const options = (projects || []).map((p) => `<option value="${escape(p.id)}">${escape(p.name)}</option>`).join("");
  return `<div class="modal-backdrop" role="presentation">
    <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="new-capability-title">
      <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
        <div><div class="eyebrow">Catalog / New</div><h2 id="new-capability-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">Create a capability</h2><p class="mt-1 text-[.7rem] text-muted">Define the outcome first. Implementation can evolve behind it.</p></div>
        <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
      </div>
      <form id="new-capability-form" class="space-y-5 px-5 py-5 sm:px-6">
        <div><label class="eyebrow mb-2 block" for="nc-name">Capability name</label><input id="nc-name" name="name" class="field" required data-autofocus placeholder="e.g. Summarize customer feedback" /></div>
        <div><label class="eyebrow mb-2 block" for="nc-description">Business outcome</label><textarea id="nc-description" name="description" class="field min-h-24 resize-y" placeholder="What should this capability reliably help your team accomplish?"></textarea></div>
        <div class="grid gap-4 sm:grid-cols-2">
          <div><label class="eyebrow mb-2 block" for="nc-project">Project</label><select id="nc-project" name="project_id" class="field" required>${options || `<option value="" disabled selected>No projects yet</option>`}</select></div>
          <div><label class="eyebrow mb-2 block" for="nc-owner">Owner (user id)</label><input id="nc-owner" name="owner" class="field" placeholder="optional" /></div>
        </div>
        <div><label class="eyebrow mb-2 block" for="nc-tags">Tags (comma separated)</label><input id="nc-tags" name="tags" class="field" placeholder="research, finance" /></div>
        <p id="nc-error" class="hidden rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800"></p>
        <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end"><button type="button" class="quiet-button" data-close-modal>Cancel</button><button type="submit" class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>Create capability</button></div>
      </form>
    </section>
  </div>`;
}

export function renderReleaseModal(release, capability, summariseFailure) {
  if (!release || !release.data) {
    return `<div class="modal-backdrop" role="presentation"><section class="modal-card" role="dialog"><div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6"><div><div class="eyebrow">Release</div><h2 class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">Loading…</h2></div><button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button></div><div class="px-6 py-6 text-[.78rem] text-muted"><span class="skeleton block h-4 w-40"></span><span class="skeleton mt-3 block h-4 w-72"></span><span class="skeleton mt-3 block h-4 w-56"></span></div></section></div>`;
  }
  const r = release.data;
  const name = capability?.name || r.capability_id;
  return `<div class="modal-backdrop" role="presentation">
    <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="release-title">
      <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
        <div><div class="eyebrow">Release review · ${escape(r.environment)}</div><h2 id="release-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">${escape(name)}</h2><p class="mt-1 text-[.7rem] text-muted">v${escape(r.capability_version)} · <span class="mono">${escape(r.id)}</span> · status <span class="font-semibold text-ink">${escape(r.status)}</span></p></div>
        <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
      </div>
      <div class="space-y-5 px-5 py-5 sm:px-6">
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <div class="rounded-lg bg-paper p-3"><span class="eyebrow">Version</span><span class="mono mt-2 block text-[.8rem] font-bold">${escape(r.capability_version)}</span></div>
          <div class="rounded-lg bg-paper p-3"><span class="eyebrow">Environment</span><span class="mt-2 block text-[.8rem] font-bold">${escape(r.environment)}</span></div>
          <div class="rounded-lg bg-paper p-3"><span class="eyebrow">Created</span><span class="mt-2 block text-[.8rem] font-bold">${escape(formatRelative(r.created_at))}</span></div>
          <div class="rounded-lg bg-paper p-3"><span class="eyebrow">Status</span><span class="mt-2 block text-[.8rem] font-bold">${escape(r.status)}</span></div>
        </div>
        <div><div class="eyebrow">Manifest fingerprints</div><div class="mt-2 max-h-32 overflow-auto rounded-lg bg-paper p-3 text-[.65rem] mono text-[#50535a]">${Object.entries(r.manifest || {}).map(([key, value]) => `<div class="flex items-center gap-2"><span class="text-muted">${escape(key)}:</span> <span>${escape(typeof value === "string" ? value : JSON.stringify(value))}</span></div>`).join("") || "<span class='text-muted'>(empty manifest)</span>"}</div></div>
        <form id="vote-form" class="space-y-3">
          <label class="eyebrow block" for="vote-note">Decision note</label>
          <textarea id="vote-note" class="field min-h-20 resize-y" placeholder="Add context for the audit trail"></textarea>
          <p id="vote-error" class="hidden rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800"></p>
          <div class="flex flex-col-reverse gap-2 pt-2 sm:flex-row sm:justify-end">
            <button type="button" class="quiet-button" data-close-modal>Not now</button>
            <button type="submit" name="decision" value="reject" class="quiet-button">Reject</button>
            <button type="submit" name="decision" value="approve" class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-check"/></svg>Record approval</button>
          </div>
        </form>
      </div>
    </section>
  </div>`;
}

export const utils = { escape, formatCompact, formatInteger, formatMoney, formatPercent, formatRelative, initials, statusPill };
