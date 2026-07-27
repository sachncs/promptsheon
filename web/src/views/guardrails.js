import * as api from "../api.js";
import { escape, formatRelative, apiStatusLabel } from "../utils.js";

function pill(text, tone = "neutral") {
  return `<span class="status-pill ${tone}"><span class="status-dot"></span>${escape(text)}</span>`;
}

function severityTone(severity) {
  return ({ critical: "danger", high: "danger", medium: "warn", low: "neutral" })[severity] || "neutral";
}

function statusTone(status) {
  return ({ active: "danger", pending: "warn", resolved: "good" })[status] || "neutral";
}

function row(alert) {
  const tone = severityTone(alert.severity);
  return `<article class="panel p-5" data-alert-id="${escape(alert.id)}">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0">
        <div class="flex flex-wrap items-center gap-2">
          ${pill(alert.severity, tone)}
          ${pill(alert.status, statusTone(alert.status))}
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
    root.innerHTML = `<p class="panel p-6 text-center text-[.78rem]">${escape(apiStatusLabel(alertsRes))}</p>`;
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
  const shell = `
    <section class="flex flex-wrap items-end justify-between gap-3">
      <div>
        <div class="eyebrow">Risk surface</div>
        <h1 class="mt-2 text-[1.4rem] font-bold tracking-[-.04em]">Guardrails</h1>
        <p class="mt-1 text-[.78rem] text-muted">Active and resolved alerts ordered by severity. Resolve to clear from this view; rule CRUD lives in <a class="font-semibold text-ink hover:text-accent" href="#/operations/alerts">Operations / Alerts</a>.</p>
      </div>
      <div class="flex shrink-0 items-center gap-2">
        ${pill(`${active} active`, active > 0 ? "danger" : "good")}
        ${pill(`${total} total`, "neutral")}
      </div>
    </section>
    <section class="mt-5 space-y-3">
      ${total === 0 ? `<div class="rounded-xl border border-dashed border-line bg-paper p-8 text-center"><p class="text-[.78rem] font-bold">All clear</p><p class="mt-1 text-[.66rem] text-muted">No signals fired in the current window.</p></div>` : ""}
      ${(["critical", "high", "medium", "low", "informational"]).map((sev) => grouped[sev].length ? `<section><div class="eyebrow mb-2">${escape(sev)} (${grouped[sev].length})</div>${grouped[sev].map(row).join("")}</section>` : "").join("")}
    </section>
  `;
  root.innerHTML = shell;

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
  return shell;
}
