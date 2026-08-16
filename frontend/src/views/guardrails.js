// src/views/guardrails.js — alerts view.
//
// Lists every alert grouped by severity, with inline resolve
// controls. Surfaces the count of active vs. total signals as pills
// in the page header so a glance at the URL tells the operator
// whether anything needs their attention.

import * as api from "../api.js";
import { escape, formatRelative, apiStatusLabel } from "../utils.js";
import { statusPill, pageHeader, panel, emptyState, errorState, inlineBanner } from "../ui.js";

const SEVERITY_TONES = { critical: "danger", high: "danger", medium: "warn", low: "neutral", informational: "info" };
const STATUS_TONES = { active: "danger", pending: "warn", resolved: "good" };

function severityTone(s) { return SEVERITY_TONES[s] || "neutral"; }
function statusTone(s) { return STATUS_TONES[s] || "neutral"; }

function alertCard(alert) {
  const sev = severityTone(alert.severity);
  return `<article class="panel p-5" data-alert-id="${escape(alert.id)}">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0">
        <div class="flex flex-wrap items-center gap-2">
          ${statusPill(alert.severity, sev)}
          ${statusPill(alert.status, statusTone(alert.status))}
          <span class="text-[.7rem] text-muted">Triggered ${escape(formatRelative(alert.triggered_at))}</span>
        </div>
        <h3 class="mt-2 text-[1rem] font-bold">${escape(alert.rule_name || alert.message || "Alert")}</h3>
        <p class="mt-1 text-[.74rem] text-muted">${escape(alert.message || "")}</p>
        ${alert.resolved_at ? `<p class="mt-1 text-[.62rem] text-muted">Resolved ${escape(formatRelative(alert.resolved_at))}</p>` : ""}
        ${alert.details && Object.keys(alert.details).length ? `<details class="mt-3"><summary class="cursor-pointer text-[.66rem] text-muted">Details</summary><pre class="mt-2 overflow-x-auto rounded-lg bg-paper p-3 text-[.62rem] mono">${escape(JSON.stringify(alert.details, null, 2))}</pre></details>` : ""}
      </div>
      <div class="flex shrink-0 items-center gap-2">
        ${alert.status !== "resolved" ? `<button type="button" data-resolve-alert class="quiet-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-check"/></svg>Resolve</button>` : ""}
        <a href="#/operations/alerts" class="quiet-button !text-[.66rem]">Rule config</a>
      </div>
    </div>
  </article>`;
}

export async function renderGuardrails(route) {
  const root = window.document.getElementById("view");
  if (!root) return "";
  const { loadSettings } = await import("../settings.js");
  if (!loadSettings().apiKey) {
    const { renderConnectPrompt } = await import("./index.js");
    const html = renderConnectPrompt("Guardrails data requires an API key. Open Connection to paste one.");
    root.innerHTML = html;
    return html;
  }
  root.innerHTML = `<section class="panel p-6"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
  const alertsRes = await api.listAlerts();
  if (!alertsRes.ok) {
    root.innerHTML = `<div class="panel p-6">${errorState(alertsRes)}</div>`;
    return "";
  }
  const all = alertsRes.data || [];
  const grouped = { critical: [], high: [], medium: [], low: [], informational: [] };
  for (const a of all) {
    const sev = a.severity || "informational";
    (grouped[sev] || grouped.informational).push(a);
  }
  const total = all.length;
  const active = all.filter((a) => a.status === "active" || a.status === "pending").length;

  const header = pageHeader({
    eyebrow: "Risk surface",
    title: "Guardrails",
    description: `Active and resolved alerts ordered by severity. Resolve to clear from this view; rule CRUD lives in <a class="font-semibold text-ink hover:text-accent" href="#/operations/alerts">Operations / Alerts</a>.`,
    actions: `
      ${statusPill(`${active} active`, active > 0 ? "danger" : "good")}
      ${statusPill(`${total} total`, "neutral")}
    `,
  });

  const sections = ["critical", "high", "medium", "low", "informational"]
    .map((sev) => {
      const items = grouped[sev];
      if (!items || !items.length) return "";
      return `<section>
        <div class="eyebrow mb-2">${escape(sev)} (${items.length})</div>
        <div class="space-y-3">${items.map(alertCard).join("")}</div>
      </section>`;
    })
    .join("");

  const html = [
    header,
    `<section class="mt-5 space-y-4">
      ${total === 0 ? inlineBanner({ tone: "good", message: "All clear — no signals fired in the current window." }) : ""}
      ${sections}
    </section>`,
  ].join("");
  root.innerHTML = html;

  root.querySelectorAll("[data-alert-id]").forEach((card) => {
    const id = card.dataset.alertId;
    card.querySelector("[data-resolve-alert]")?.addEventListener("click", async () => {
      const result = await api.resolveAlert(id);
      if (!result.ok) {
        window.alert(`Resolve failed: ${apiStatusLabel(result)}`);
        return;
      }
      card.style.opacity = "0.5";
      setTimeout(() => window.location.reload(), 400);
    });
  });
  return html;
}
