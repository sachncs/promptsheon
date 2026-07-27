import * as api from "../api.js";
import { escape, formatRelative, apiStatusLabel } from "../utils.js";
import { renderConnectPrompt } from "./index.js";

const ACTIONS = ["all", "create", "update", "delete", "activate", "rollback", "vote", "invoke"];
const ACTION_LABEL = { create: "create", update: "update", delete: "delete", activate: "activate", rollback: "rollback", vote: "vote", invoke: "invoke", resolve: "resolve", approve: "approve", reject: "reject" };

function pill(text, tone = "neutral") {
  return `<span class="status-pill ${tone} !px-2 !py-1"><span class="status-dot"></span>${escape(text)}</span>`;
}

function tone(action) {
  return ({ delete: "danger", update: "warn", create: "good", activate: "good", rollback: "danger", reject: "danger", approve: "good" })[action] || "neutral";
}

function chips(active) {
  return ACTIONS.map((action) => {
    const on = (active || "all") === action;
    const label = action === "all" ? "all" : action;
    return `<button type="button" data-audit-action="${escape(action)}" class="rounded-md ${on ? "bg-ink text-paper" : "bg-paper text-muted hover:text-ink"} px-2.5 py-1.5 text-[.66rem] font-${on ? "bold" : "semibold"}">${escape(label)}</button>`;
  }).join("");
}

function row(entry) {
  const details = entry.details || {};
  const target = (entry.resource || "").split(":").pop();
  const label = details.name || target || entry.resource;
  const subject = (entry.user_id || "api") === "api" ? "System" : entry.user_id;
  const actionLabel = ACTION_LABEL[entry.action] || entry.action;
  return `<tr class="border-t border-line/60" data-audit-row>
    <td class="py-2 pr-2 text-[.66rem] text-muted whitespace-nowrap">${escape(formatRelative(entry.timestamp))}</td>
    <td class="py-2 pr-2 text-[.66rem] font-bold whitespace-nowrap">${escape(subject)}</td>
    <td class="py-2 pr-2 text-[.66rem] whitespace-nowrap">${pill(actionLabel, tone(entry.action))}</td>
    <td class="py-2 pr-2 text-[.66rem] mono truncate max-w-[16rem]" title="${escape(entry.resource)}">${escape(entry.resource)}</td>
    <td class="py-2 pr-2 text-[.66rem] font-bold truncate max-w-[20rem]">${escape(label)}</td>
  </tr>`;
}

function renderTable(audit, action) {
  if (!audit || !audit.ok || !audit.data) {
    if (audit?.status === 429) return `<p class="text-[.7rem]">Audit log rate-limited. Retrying automatically.</p>`;
    return `<p class="text-[.7rem]">Audit log unavailable${audit?.error ? ` (${escape(audit.error)})` : ""}.</p>`;
  }
  const rows = audit.data || [];
  if (!rows.length) return `<p class="text-[.7rem] text-muted">No entries match this filter.</p>`;
  return `<div class="overflow-x-auto rounded-xl border border-line"><table class="w-full text-[.7rem]"><thead><tr class="bg-paper text-left text-[.6rem] uppercase tracking-wider text-muted"><th class="px-3 py-2 font-bold">When</th><th class="px-3 py-2 font-bold">Who</th><th class="px-3 py-2 font-bold">Action</th><th class="px-3 py-2 font-bold">Resource</th><th class="px-3 py-2 font-bold">Subject</th></tr></thead><tbody>${rows.map(row).join("")}</tbody></table></div>
  <p class="mt-3 text-[.62rem] text-muted">Showing ${rows.length} entries. Latest first.</p>`;
}

function renderVerifyResult(result) {
  if (!result) return "";
  if (!result.ok || !result.data) {
    const tone = result.status === 429 ? "warn" : "danger";
    return `<div class="rounded-lg bg-${tone}-50 px-3 py-2 text-[.68rem] text-${tone}-800">Verify failed: ${escape(apiStatusLabel(result))}</div>`;
  }
  const v = result.data || {};
  const tone = v.ok ? "good" : "danger";
  const reason = v.reason ? ` (${escape(v.reason)})` : "";
  return `<div class="rounded-lg bg-${tone}-50 px-3 py-2 text-[.68rem] text-${tone}-800">
    <span class="font-bold">${v.ok ? "Chain verified" : "Chain mismatch"}${reason}</span>${v.last_hash ? ` · last hash <span class="mono">${escape((v.last_hash || "").slice(0, 12))}…</span>` : ""}${v.last_row_id != null ? ` · tail <span class="mono">#${escape(String(v.last_row_id))}</span>` : ""}
  </div>`;
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
  root.innerHTML = `<section class="panel p-6"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;

  const audit = await api.listAudit({ limit: 100, action: action === "all" ? undefined : action, resource: resource || undefined });
  let verifyResult = null;
  let verifyRunning = false;
  let exportResult = { status: "idle" };
  const shell = `
    <section class="flex flex-wrap items-end justify-between gap-3">
      <div>
        <div class="eyebrow">Compliance</div>
        <h1 class="mt-2 text-[1.4rem] font-bold tracking-[-.04em]">Audit trail</h1>
        <p class="mt-1 text-[.78rem] text-muted">Hash-chained ledger of every state transition. Filter by action or inspect the row details to learn more.</p>
      </div>
      <div class="flex shrink-0 items-center gap-2">
        <button id="audit-verify" class="quiet-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-verify"/></svg>Verify chain</button>
        <button id="audit-export" class="quiet-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-download"/></svg>CSV export</button>
      </div>
    </section>
    <section class="panel p-5 sm:p-6 mt-5">
      <div class="flex flex-wrap items-end gap-3">
        <div>
          <div class="eyebrow mb-2">Action filter</div>
          <div class="flex flex-wrap items-center gap-1 rounded-lg bg-paper p-1" data-audit-actions>${chips(action)}</div>
        </div>
        <div class="flex-1 min-w-[200px]">
          <div class="eyebrow mb-2">Resource contains</div>
          <input id="audit-resource" class="field" value="${escape(resource)}" placeholder="e.g. capability: or release:" />
        </div>
      </div>
      <div id="audit-verify-slot" class="mt-3"></div>
      <div id="audit-export-slot" class="mt-3"></div>
    </section>
    <section class="panel p-5 sm:p-6 mt-5"><div id="audit-table">${renderTable(audit, action)}</div></section>
  `;
  root.innerHTML = shell;

  root.querySelectorAll("[data-audit-action]").forEach((btn) => {
    btn.addEventListener("click", () => {
      const next = btn.dataset.auditAction;
      const nextHash = next === "all" ? `#/audit${resource ? `?resource=${encodeURIComponent(resource)}` : ""}` : `#/audit?action=${encodeURIComponent(next)}${resource ? `&resource=${encodeURIComponent(resource)}` : ""}`;
      window.location.hash = nextHash;
      window.location.reload();
    });
  });

  const resourceInput = root.querySelector("#audit-resource");
  resourceInput?.addEventListener("change", () => {
    const next = resourceInput.value.trim();
    const action = btn?.dataset.auditAction || "all";
    window.location.hash = `#/audit${action === "all" ? "" : `?action=${encodeURIComponent(action)}`}${next ? `${action === "all" ? "?" : "&"}resource=${encodeURIComponent(next)}` : ""}`;
    window.location.reload();
  });

  root.querySelector("#audit-verify")?.addEventListener("click", async () => {
    const slot = root.querySelector("#audit-verify-slot");
    slot.innerHTML = `<div class="rounded-lg bg-paper px-3 py-2 text-[.68rem] text-muted">Verifying chain…</div>`;
    verifyResult = await api.verifyAuditChain();
    slot.innerHTML = renderVerifyResult(verifyResult);
  });

  root.querySelector("#audit-export")?.addEventListener("click", async () => {
    const slot = root.querySelector("#audit-export-slot");
    slot.innerHTML = `<div class="rounded-lg bg-paper px-3 py-2 text-[.68rem] text-muted">Building CSV…</div>`;
    const result = await api.exportAudit("csv");
    if (!result.ok || !result.blob) {
      slot.innerHTML = `<div class="rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800">Export failed: ${escape(apiStatusLabel(result))}</div>`;
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
    slot.innerHTML = `<div class="rounded-lg bg-lime/15 px-3 py-2 text-[.68rem] text-[#52632d]">CSV downloaded.</div>`;
  });

  return shell;
}
