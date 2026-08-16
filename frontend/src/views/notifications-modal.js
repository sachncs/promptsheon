// Notifications inbox modal. Opened from the header bell. Shows
// active alerts and the most recent activity from the audit trail.

import * as api from "../api.js";
import { escape, formatRelative, apiStatusLabel } from "../utils.js";
import { statusPill, emptyState, errorState } from "../ui.js";
import { openModal } from "../dialog.js";

const ACTION_TONES = { delete: "danger", update: "warn", create: "good", activate: "good", rollback: "danger" };
const SEVERITY_TONES = { critical: "danger", high: "danger", medium: "warn", low: "neutral" };

function actionTone(action) { return ACTION_TONES[action] || "neutral"; }
function severityTone(s) { return SEVERITY_TONES[s] || "neutral"; }

function auditRow(entry) {
  const details = entry.details || {};
  const target = (entry.resource || "").split(":").pop();
  const label = details.name || target || entry.resource;
  const subject = (entry.user_id || "api") === "api" ? "System" : entry.user_id;
  return `<a href="#/audit?action=${encodeURIComponent(entry.action || "")}" class="flex items-start gap-3 rounded-lg px-2 py-2 transition hover:bg-paper/60">
    <span class="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-paper text-[#5c5e63]"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-pulse"/></svg></span>
    <div class="min-w-0 flex-1"><p class="text-[.74rem] leading-5"><span class="font-bold">${escape(subject)}</span> · <span class="font-semibold">${escape(entry.action)}</span> · <span class="font-bold">${escape(label)}</span></p><p class="mt-0.5 text-[.63rem] text-muted">${escape(formatRelative(entry.timestamp))} <span class="mx-1 text-[#c1c2bd]">·</span> <span class="mono">${escape(entry.resource)}</span></p></div>
    ${statusPill(entry.action, actionTone(entry.action))}
  </a>`;
}

function alertRow(alert) {
  return `<a href="#/guardrails" class="flex items-start gap-3 rounded-lg bg-rose-50 px-2 py-2 transition hover:bg-rose-100">
    <span class="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-rose-100 text-rose-700"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-warning"/></svg></span>
    <div class="min-w-0 flex-1"><p class="text-[.74rem] font-bold">${escape(alert.rule_name || alert.message || "Alert")}</p><p class="mt-0.5 text-[.62rem] text-muted">${escape(alert.message || "")}</p><p class="mt-1 text-[.62rem] text-muted">Triggered ${escape(formatRelative(alert.triggered_at))}</p></div>
    ${statusPill(alert.severity, severityTone(alert.severity))}
  </a>`;
}

export async function openNotificationsModal(root) {
  if (!root) return;
  const modal = openModal({
    title: "Notifications",
    subtitle: "Active alerts and the most recent activity from the audit trail.",
    body: `
      <section>
        <div class="flex items-center justify-between"><div class="eyebrow">Active alerts</div><span id="notif-alerts-count" class="text-[.62rem] text-muted">—</span></div>
        <div id="notif-alerts" class="mt-3 space-y-2"><div class="skeleton h-12 w-full"></div></div>
      </section>
      <section>
        <div class="flex items-center justify-between"><div class="eyebrow">Recent activity</div><a href="#/audit" class="text-[.62rem] text-muted hover:text-ink">Open audit →</a></div>
        <div id="notif-audit" class="mt-3 space-y-1"><div class="skeleton h-10 w-full"></div></div>
      </section>
    `,
    size: "wide",
  });

  const { loadSettings } = await import("../settings.js");
  if (!loadSettings().apiKey) return;

  const [alertsRes, auditRes] = await Promise.all([
    api.listAlerts().catch((e) => ({ ok: false, error: String(e?.message || e) })),
    api.listAudit({ limit: 8 }).catch((e) => ({ ok: false, error: String(e?.message || e) })),
  ]);

  const alertsBox = modal.root.querySelector("#notif-alerts");
  const alertsCount = modal.root.querySelector("#notif-alerts-count");
  if (alertsRes.ok && Array.isArray(alertsRes.data) && alertsRes.data.length) {
    const active = alertsRes.data.filter((a) => a.status === "active" || a.status === "pending");
    if (active.length) {
      alertsBox.innerHTML = active.slice(0, 4).map(alertRow).join("");
      alertsCount.textContent = `${active.length} firing`;
    } else {
      alertsBox.innerHTML = emptyState("No active alerts. All clear.", { icon: "icon-shield" });
      alertsCount.textContent = "0 firing";
    }
  } else {
    alertsBox.innerHTML = errorState(alertsRes);
    alertsCount.textContent = "—";
  }

  const auditBox = modal.root.querySelector("#notif-audit");
  if (auditRes.ok && Array.isArray(auditRes.data) && auditRes.data.length) {
    auditBox.innerHTML = auditRes.data.slice(0, 6).map(auditRow).join("");
  } else {
    auditBox.innerHTML = errorState(auditRes);
  }
}
