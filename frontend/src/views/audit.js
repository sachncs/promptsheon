// src/views/audit.js — full audit-trail view.
//
// Reachable at #/audit; supports ?action= and ?resource= filters.
// The audit log requires a connection API key, so the view surfaces
// the connect prompt instead of trying to load.

import * as api from "../api.js";
import { escape, formatRelative, apiStatusLabel } from "../utils.js";
import { renderConnectPrompt } from "./index.js";
import { statusPill, pageHeader, panel, chipGroup, dataTable, emptyState, errorState, inlineBanner } from "../ui.js";

const ACTIONS = ["all", "create", "update", "delete", "activate", "rollback", "vote", "invoke"];
const ACTION_LABEL = {
  create: "create", update: "update", delete: "delete",
  activate: "activate", rollback: "rollback", vote: "vote", invoke: "invoke",
  resolve: "resolve", approve: "approve", reject: "reject",
};
const ACTION_TONES = {
  delete: "danger", update: "warn", create: "good",
  activate: "good", rollback: "danger", reject: "danger", approve: "good",
  vote: "neutral", invoke: "neutral", resolve: "good",
};

function actionTone(action) { return ACTION_TONES[action] || "neutral"; }

function renderTable(audit) {
  if (!audit || !audit.ok || !audit.data) {
    if (audit?.status === 429) return inlineBanner({ tone: "warn", message: "Audit log rate-limited. Retrying automatically." });
    return errorState(audit, { prefix: "Audit log unavailable" });
  }
  const rows = audit.data || [];
  if (!rows.length) return emptyState("No entries match this filter.", { icon: "icon-scroll" });

  const subjectFor = (entry) => {
    const details = entry.details || {};
    const target = (entry.resource || "").split(":").pop();
    return details.name || target || entry.resource;
  };

  const table = dataTable({
    columns: [
      { key: "when", label: "When", render: (entry) => `<span class="text-muted">${escape(formatRelative(entry.timestamp))}</span>` },
      { key: "who", label: "Who", render: (entry) => `<span class="font-bold">${escape((entry.user_id || "api") === "api" ? "System" : entry.user_id)}</span>` },
      { key: "action", label: "Action", render: (entry) => statusPill(ACTION_LABEL[entry.action] || entry.action, actionTone(entry.action)) },
      { key: "resource", label: "Resource", render: (entry) => `<span class="mono truncate max-w-[16rem] inline-block align-middle" title="${escape(entry.resource)}">${escape(entry.resource)}</span>` },
      { key: "label", label: "Subject", render: (entry) => `<span class="font-bold truncate max-w-[20rem] inline-block align-middle">${escape(subjectFor(entry))}</span>` },
    ],
    rows,
    emptyMessage: "No entries match this filter.",
    emptyIcon: "icon-scroll",
  });
  return `${table}<p class="mt-3 text-[.62rem] text-muted">Showing ${rows.length} entries. Latest first.</p>`;
}

function renderVerifyResult(result) {
  if (!result) return "";
  if (!result.ok || !result.data) {
    const tone = result.status === 429 ? "warn" : "danger";
    return inlineBanner({ tone, message: `Verify failed: ${apiStatusLabel(result)}` });
  }
  const v = result.data || {};
  const tone = v.ok ? "good" : "danger";
  const reason = v.reason ? ` (${v.reason})` : "";
  const message = `<span class="font-bold">${v.ok ? "Chain verified" : "Chain mismatch"}${escape(reason)}</span>${v.last_hash ? ` · last hash <span class="mono">${escape((v.last_hash || "").slice(0, 12))}…</span>` : ""}${v.last_row_id != null ? ` · tail <span class="mono">#${escape(String(v.last_row_id))}</span>` : ""}`;
  return inlineBanner({ tone, message });
}

export async function renderAudit(route) {
  const root = window.document.getElementById("view");
  if (!root) return "";
  const { loadSettings } = await import("../settings.js");
  if (!loadSettings().apiKey) {
    const html = renderConnectPrompt("The audit log requires an API key. Open Connection to paste one.");
    root.innerHTML = html;
    return html;
  }
  const action = route?.query?.action || "all";
  const resource = route?.query?.resource || "";
  root.innerHTML = skeletonShell();

  const audit = await api.listAudit({ limit: 100, action: action === "all" ? undefined : action, resource: resource || undefined });

  const actionsHtml = [
    pageHeader({
      eyebrow: "Compliance",
      title: "Audit trail",
      description: "Hash-chained ledger of every state transition. Filter by action or inspect the row details to learn more.",
      actions: [
        `<button id="audit-verify" class="quiet-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-verify"/></svg>Verify chain</button>`,
        `<button id="audit-export" class="quiet-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-download"/></svg>CSV export</button>`,
      ].join(""),
    }),
    panel({
      eyebrow: "Filter",
      title: "Find entries",
      body: [
        `<div class="flex flex-wrap items-end gap-3">`,
        `<div><div class="field-label">Action</div>${chipGroup(ACTIONS.map((a) => ({ key: a, label: a })), { activeKey: action, onClickDataAttr: "data-audit-action" })}</div>`,
        `<div class="flex-1 min-w-[200px]"><div class="field-label">Resource contains</div><input id="audit-resource" class="field" value="${escape(resource)}" placeholder="e.g. capability: or release:" /></div>`,
        `</div>`,
        `<div id="audit-verify-slot" class="mt-3"></div>`,
        `<div id="audit-export-slot" class="mt-3"></div>`,
      ].join(""),
    }),
    panel({ title: "Entries", body: `<div id="audit-table">${renderTable(audit)}</div>` }),
  ].join("");

  root.innerHTML = actionsHtml;

  root.querySelectorAll("[data-audit-action]").forEach((btn) => {
    btn.addEventListener("click", () => {
      const next = btn.dataset.auditAction;
      const nextHash = next === "all"
        ? `#/audit${resource ? `?resource=${encodeURIComponent(resource)}` : ""}`
        : `#/audit?action=${encodeURIComponent(next)}${resource ? `&resource=${encodeURIComponent(resource)}` : ""}`;
      window.location.hash = nextHash;
      window.location.reload();
    });
  });

  const resourceInput = root.querySelector("#audit-resource");
  resourceInput?.addEventListener("change", () => {
    const next = resourceInput.value.trim();
    const a = action;
    const params = [];
    if (a !== "all") params.push(`action=${encodeURIComponent(a)}`);
    if (next) params.push(`resource=${encodeURIComponent(next)}`);
    const qs = params.length ? `?${params.join("&")}` : "";
    window.location.hash = `#/audit${qs}`;
    window.location.reload();
  });

  root.querySelector("#audit-verify")?.addEventListener("click", async () => {
    const slot = root.querySelector("#audit-verify-slot");
    slot.innerHTML = inlineBanner({ message: "Verifying chain…" });
    const result = await api.verifyAuditChain();
    slot.innerHTML = renderVerifyResult(result);
  });

  root.querySelector("#audit-export")?.addEventListener("click", async () => {
    const slot = root.querySelector("#audit-export-slot");
    slot.innerHTML = inlineBanner({ message: "Building CSV…" });
    const result = await api.exportAudit("csv");
    if (!result.ok || !result.blob) {
      slot.innerHTML = inlineBanner({ tone: "danger", message: `Export failed: ${apiStatusLabel(result)}` });
      return;
    }
    const url = URL.createObjectURL(result.blob);
    const a = document.createElement("a");
    a.href = url;
    const stamp = new Date().toISOString().slice(0, 10);
    a.download = `promptsheon-audit-${stamp}.csv`;
    document.body.append(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 2000);
    slot.innerHTML = inlineBanner({ tone: "good", message: "CSV downloaded." });
  });

  return actionsHtml;
}

function skeletonShell() {
  return `<section class="panel p-6"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
}
