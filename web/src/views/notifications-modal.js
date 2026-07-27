import * as api from "../api.js";
import { escape, formatRelative, apiStatusLabel } from "../utils.js";

function pill(text, tone = "neutral") {
  return `<span class="status-pill ${tone} !px-2 !py-1"><span class="status-dot"></span>${escape(text)}</span>`;
}

function toneFor(action) {
  return ({ delete: "danger", update: "warn", create: "good", activate: "good", rollback: "danger" })[action] || "neutral";
}

function auditRow(entry) {
  const details = entry.details || {};
  const target = (entry.resource || "").split(":").pop();
  const label = details.name || target || entry.resource;
  const subject = (entry.user_id || "api") === "api" ? "System" : entry.user_id;
  return `<a href="#/audit?action=${encodeURIComponent(entry.action || "")}" class="flex items-start gap-3 rounded-lg px-2 py-2 transition hover:bg-paper/60">
    <span class="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-paper text-[#5c5e63]"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-pulse"/></svg></span>
    <div class="min-w-0 flex-1"><p class="text-[.74rem] leading-5"><span class="font-bold">${escape(subject)}</span> · <span class="font-semibold">${escape(entry.action)}</span> · <span class="font-bold">${escape(label)}</span></p><p class="mt-0.5 text-[.63rem] text-muted">${escape(formatRelative(entry.timestamp))} <span class="mx-1 text-[#c1c2bd]">·</span> <span class="mono">${escape(entry.resource)}</span></p></div>
    ${pill(entry.action, toneFor(entry.action))}
  </a>`;
}

function alertRow(alert) {
  const sevTone = ({ critical: "danger", high: "danger", medium: "warn", low: "neutral" })[alert.severity] || "neutral";
  return `<a href="#/guardrails" class="flex items-start gap-3 rounded-lg bg-rose-50 px-2 py-2 transition hover:bg-rose-100">
    <span class="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-rose-100 text-rose-700"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-warning"/></svg></span>
    <div class="min-w-0 flex-1"><p class="text-[.74rem] font-bold">${escape(alert.rule_name || alert.message || "Alert")}</p><p class="mt-0.5 text-[.62rem] text-muted">${escape(alert.message || "")}</p><p class="mt-1 text-[.62rem] text-muted">Triggered ${escape(formatRelative(alert.triggered_at))}</p></div>
    ${pill(alert.severity, sevTone)}
  </a>`;
}

export async function openNotificationsModal(root) {
  if (!root) return;
  const shell = `<div class="modal-backdrop" role="presentation">
    <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="notif-title">
      <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
        <div><div class="eyebrow">Inbox</div><h2 id="notif-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">Notifications</h2><p class="mt-1 text-[.7rem] text-muted">Active alerts and the most recent activity from the audit trail.</p></div>
        <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
      </div>
      <div class="px-5 py-5 sm:px-6 space-y-5 max-h-[70vh] overflow-y-auto">
        <section>
          <div class="flex items-center justify-between"><div class="eyebrow">Active alerts</div><span id="notif-alerts-count" class="text-[.62rem] text-muted">—</span></div>
          <div id="notif-alerts" class="mt-3 space-y-2"><div class="skeleton h-12 w-full"></div></div>
        </section>
        <section>
          <div class="flex items-center justify-between"><div class="eyebrow">Recent activity</div><a href="#/audit" class="text-[.62rem] text-muted hover:text-ink">Open audit →</a></div>
          <div id="notif-audit" class="mt-3 space-y-1"><div class="skeleton h-10 w-full"></div></div>
        </section>
      </div>
    </section>
  </div>`;
  root.innerHTML = shell;
  root.querySelector("[data-close-modal]")?.addEventListener("click", () => root.replaceChildren());

  const { loadSettings } = await import("../settings.js");
  if (!loadSettings().apiKey) {
    return;
  }

  const [alertsRes, auditRes] = await Promise.all([
    api.listAlerts().catch((e) => ({ ok: false, error: String(e?.message || e) })),
    api.listAudit({ limit: 8 }).catch((e) => ({ ok: false, error: String(e?.message || e) }))
  ]);

  const alertsBox = root.querySelector("#notif-alerts");
  const alertsCount = root.querySelector("#notif-alerts-count");
  if (alertsRes.ok && Array.isArray(alertsRes.data) && alertsRes.data.length) {
    const active = alertsRes.data.filter((a) => a.status === "active" || a.status === "pending");
    if (active.length) {
      alertsBox.innerHTML = active.slice(0, 4).map(alertRow).join("");
      alertsCount.textContent = `${active.length} firing`;
    } else {
      alertsBox.innerHTML = `<p class="rounded-lg border border-dashed border-line bg-paper p-3 text-center text-[.7rem] text-muted">No active alerts. All clear.</p>`;
      alertsCount.textContent = "0 firing";
    }
  } else {
    alertsBox.innerHTML = `<p class="text-[.68rem] text-muted">${escape(apiStatusLabel(alertsRes)) || "Alerts unavailable."}</p>`;
    alertsCount.textContent = "—";
  }

  const auditBox = root.querySelector("#notif-audit");
  if (auditRes.ok && Array.isArray(auditRes.data) && auditRes.data.length) {
    auditBox.innerHTML = auditRes.data.slice(0, 6).map(auditRow).join("");
  } else {
    auditBox.innerHTML = `<p class="text-[.68rem] text-muted">${escape(apiStatusLabel(auditRes)) || "Audit log unavailable."}</p>`;
  }
}
