// src/views/capability-detail.js — capability detail page.
//
// Surfaces the capability, its versions, contract, datasets, eval
// preconditions, reputation gauge, and self-evolution toggle. The
// page uses ui.js primitives for the page header, panels, status
// pills, gauges, and inline banners so the detail surfaces match the
// dashboard's standard layout.

import * as api from "../api.js";
import { escape, formatPercent, formatRelative, apiStatusLabel } from "../utils.js";
import { ownerName } from "../state/owners.js";
import { statusPill, pageHeader, panel, dataTable, emptyState, errorState, inlineBanner, progressBar } from "../ui.js";

const skeletons = {
  header: () => `<div class="panel p-6"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-10 w-80"></div><div class="skeleton mt-3 h-4 w-64"></div></div>`,
  versions: () => `<div class="panel p-5 sm:p-6"><div class="skeleton h-3 w-24"></div><div class="skeleton mt-4 h-12 w-full"></div><div class="skeleton mt-4 h-12 w-full"></div></div>`,
  contract: () => `<div class="panel p-5 sm:p-6"><div class="skeleton h-3 w-28"></div><div class="skeleton mt-4 h-8 w-full"></div><div class="skeleton mt-3 h-8 w-full"></div></div>`,
  reputation: () => `<div class="panel p-5"><div class="skeleton h-3 w-24"></div><div class="skeleton mt-4 h-20 w-32"></div></div>`,
};

function contractBlock(contract) {
  if (!contract) return `<p class="mt-2 text-[.68rem] text-muted">No contract set. Capabilities without a contract cannot be auto-promoted by the recommendation engine.</p>`;
  const slo = contract.slo_target || {};
  const blast = contract.blast_radius || "—";
  const schema = (label, value) => `<details class="mt-3 rounded-lg bg-paper p-3 text-[.66rem]"><summary class="cursor-pointer text-[.72rem] font-bold">${escape(label)}</summary><pre class="mt-2 overflow-x-auto text-[.62rem] mono">${escape(JSON.stringify(value, null, 2))}</pre></details>`;
  const optional = (label, value) => `<div class="rounded-lg bg-paper p-3"><span class="eyebrow">${escape(label)}</span><span class="mt-2 block text-[.8rem] font-bold break-words">${escape(value)}</span></div>`;
  return `<div class="mt-3 grid gap-3 sm:grid-cols-2">
    ${optional("Success rubric", contract.success_rubric || "—")}
    ${optional("Blast radius", blast)}
    ${optional("Auto-promote", contract.auto_promotable ? "Yes" : "No")}
    ${optional("SLO p95", slo.max_p95_latency_ms ? `${slo.max_p95_latency_ms}ms` : "—")}
    ${optional("SLO success rate", slo.min_success_rate ? formatPercent(slo.min_success_rate * 100) : "—")}
    ${optional("SLO hallucination cap", slo.max_hallucination_rate ? formatPercent(slo.max_hallucination_rate * 100) : "—")}
  </div>
  ${contract.input_schema ? schema("Input schema", contract.input_schema) : ""}
  ${contract.output_schema ? schema("Output schema", contract.output_schema) : ""}`;
}

function renderVersionsTable(versions, currentId) {
  if (!versions?.length) return `<p class="mt-3 text-[.68rem] text-muted">No versions yet. Create the first version to enable releases.</p>`;
  return dataTable({
    columns: [
      { key: "version", label: "Version", render: (v) => `<a href="#/versions/${escape(v.id)}" class="mono font-bold hover:underline">v${escape(v.version)}</a>` },
      { key: "manifest", label: "Manifest hash", render: (v) => `<span class="mono truncate max-w-[12rem] inline-block align-middle">${escape((v.manifest_hash || "").slice(0, 12))}…</span>` },
      { key: "created", label: "Created", render: (v) => `<span class="text-muted">${escape(formatRelative(v.created_at))}</span>` },
      { key: "by", label: "By", render: (v) => `<span class="text-muted">${escape(v.created_by || "—")}</span>` },
      { key: "diff", label: "", align: "right", render: (v) => `<button type="button" data-pick-diff="${escape(v.id)}" data-version="${escape(v.version)}" class="quiet-button !text-[.6rem]">Diff against v${escape(v.version)}</button>` },
    ],
    rows: versions.slice(0, 30),
    emptyMessage: "No versions yet.",
    emptyIcon: "icon-layers",
  }) + `<p class="mt-3 text-[.62rem] text-muted">Pick any version above to diff it against another version of the same capability.</p>`;
}

function reputationGauge(rep) {
  if (!rep) return `<p class="mt-3 text-[.68rem] text-muted">No reputation data yet (no evaluations have run).</p>`;
  const score = Math.max(0, Math.min(1, rep.trust_score || 0));
  const r = 28;
  const cx = 32; const cy = 32;
  const dash = 2 * Math.PI * r;
  const filled = dash * score;
  return `<div class="mt-3 flex items-center gap-4">
    <svg viewBox="0 0 64 64" class="h-20 w-20">
      <defs>
        <linearGradient id="gauge" x1="0%" y1="0%" x2="100%" y2="0%">
          <stop offset="0%" stop-color="#c9f36a" />
          <stop offset="100%" stop-color="#6878ff" />
        </linearGradient>
      </defs>
      <circle cx="${cx}" cy="${cy}" r="${r}" fill="none" stroke="#e6e6e1" stroke-width="8"></circle>
      <circle cx="${cx}" cy="${cy}" r="${r}" fill="none" stroke="url(#gauge)" stroke-width="8" stroke-linecap="round" stroke-dasharray="${filled} ${dash - filled}" transform="rotate(-90 ${cx} ${cy})"></circle>
    </svg>
    <div>
      <p class="metric-value">${escape((score * 100).toFixed(0))}<span class="text-[.9rem] text-muted">%</span></p>
      <p class="text-[.7rem] text-muted">${escape(rep.label || "Trust score")}</p>
      <p class="text-[.62rem] text-muted">${escape(rep.history || "")}</p>
    </div>
  </div>`;
}

function header(capability) {
  return pageHeader({
    eyebrow: "Capability",
    title: capability.name,
    description: capability.description || "No description provided.",
    actions: `
      ${statusPill(ownerName(capability.owner) || "—", "neutral")}
      <button data-action="edit-capability" class="quiet-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-edit"/></svg>Edit</button>
      <button data-action="delete-capability" class="danger-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-trash"/></svg>Delete</button>
    `,
  });
}

function versionsCard(versions, latestVersion) {
  const latestPill = latestVersion != null ? statusPill(`Latest v${latestVersion}`, "good") : "";
  return panel({
    eyebrow: "Versions",
    rightSlot: `${latestPill}<button data-action="new-version" class="quiet-button !text-[.66rem]"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>New version</button>`,
    body: renderVersionsTable(versions),
  });
}

function datasetsCard(datasets) {
  const rows = (datasets || []).map((d) => `<li class="flex items-center justify-between border-t border-line/60 px-5 py-3" data-dataset-id="${escape(d.id)}">
    <div>
      <p class="text-[.78rem] font-bold">${escape(d.name || "—")}</p>
      <p class="text-[.62rem] text-muted mono">${escape(d.id)}</p>
    </div>
    <div class="flex items-center gap-2">
      <button data-dataset-cases="${escape(d.id)}" class="quiet-button !text-[.62rem]">Cases</button>
      <button data-dataset-delete="${escape(d.id)}" data-dataset-name="${escape(d.name || d.id)}" class="danger-button !text-[.62rem]">Delete</button>
    </div>
  </li>`).join("");
  return panel({
    eyebrow: "Datasets",
    rightSlot: `<button data-action="new-dataset" class="quiet-button !text-[.66rem]"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>New dataset</button>`,
    body: datasets?.length ? `<ul class="mt-3 -mx-5">${rows}</ul>` : `<p class="mt-3 text-[.68rem] text-muted">No datasets yet. Create one to start running eval cases.</p>`,
  });
}

function preconditionsCard(preconditions) {
  const rows = (preconditions || []).map((p) => `<li class="flex items-center justify-between border-t border-line/60 px-5 py-3" data-precondition-id="${escape(p.id)}">
    <div>
      <p class="text-[.78rem] font-bold">${escape(p.name || "—")}</p>
      <p class="text-[.66rem] text-muted">${escape(p.command || "")}</p>
      <p class="text-[.62rem] text-muted mono">${escape(p.id)}</p>
    </div>
    <div class="flex items-center gap-2">
      <button data-precondition-edit="${escape(p.id)}" class="quiet-button !text-[.62rem]">Edit</button>
      <button data-precondition-delete="${escape(p.id)}" data-precondition-name="${escape(p.name || p.id)}" class="danger-button !text-[.62rem]">Delete</button>
    </div>
  </li>`).join("");
  return panel({
    eyebrow: "Preconditions",
    rightSlot: `<button data-action="new-precondition" class="quiet-button !text-[.66rem]"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>New precondition</button>`,
    body: preconditions?.length ? `<ul class="mt-3 -mx-5">${rows}</ul>` : `<p class="mt-3 text-[.68rem] text-muted">No preconditions yet. Add command hooks that must pass before a release can activate.</p>`,
  });
}

function contractCard(contract) {
  return panel({
    eyebrow: "Capability contract",
    rightSlot: `<button data-action="edit-contract" class="quiet-button !text-[.66rem]"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-edit"/></svg>Edit</button>`,
    body: contractBlock(contract),
  });
}

function selfEvolveCard(config) {
  const enabled = !!config?.enabled;
  const optional = (label, value) => `<div class="rounded-lg bg-paper p-3"><span class="eyebrow">${escape(label)}</span><span class="mt-2 block text-[.8rem] font-bold break-words">${escape(value)}</span></div>`;
  return panel({
    eyebrow: "Self-evolution",
    rightSlot: statusPill(enabled ? "on" : "off", enabled ? "good" : "neutral"),
    body: `
      <div class="mt-3 grid gap-3 sm:grid-cols-2">
        ${optional("Min score", config?.min_score ?? "—")}
        ${optional("Max revisions", config?.max_revisions ?? "—")}
        ${optional("Cooldown (s)", config?.cooldown_sec ?? "—")}
        ${optional("Target env", config?.target_env ?? "—")}
      </div>
      <div class="mt-3 flex justify-end"><button data-action="edit-self-evolve" class="quiet-button !text-[.66rem]"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-edit"/></svg>Configure</button></div>
    `,
  });
}

function reputationCard(rep) {
  return panel({ eyebrow: "Reputation", body: reputationGauge(rep) });
}

function diffCard(fromVersion, toVersion, diff) {
  if (!diff) return `<div class="panel p-5 sm:p-6"><div class="eyebrow">Diff ${escape(String(fromVersion))} → ${escape(String(toVersion))}</div><p class="mt-2 text-[.68rem] text-muted">Pick two versions above to see added, removed, and changed artifacts.</p></div>`;
  const section = (title, items, tone) => `<div class="mt-3"><div class="eyebrow">${escape(title)} (${items.length})</div>${items.length ? `<ul class="mt-2 space-y-1 text-[.7rem] mono">${items.map((it) => `<li class="flex items-center gap-2 rounded-md bg-${tone}-50 px-2 py-1"><span class="font-bold">${escape(it.kind)}</span><span class="truncate text-muted">${escape(JSON.stringify(it.hash || it.old_hash || it.new_hash || ""))}</span></li>`).join("")}</ul>` : `<p class="mt-1 text-[.68rem] text-muted">None.</p>`}</div>`;
  return panel({
    eyebrow: `Diff v${fromVersion} → v${toVersion}`,
    rightSlot: `<button id="diff-clear" class="quiet-button !text-[.66rem]">Close</button>`,
    body: `
      ${section("Added", diff.added || [], "lime")}
      ${section("Removed", diff.removed || [], "rose")}
      ${section("Changed", diff.changed || [], "amber")}
    `,
  });
}

function renderSkeleton() {
  return `<section class="grid gap-5 xl:grid-cols-[minmax(0,2fr)_minmax(280px,.85fr)]">
    <div class="space-y-5">${skeletons.header()}${skeletons.versions()}${skeletons.contract()}</div>
    <div class="space-y-5">${skeletons.reputation()}</div>
  </section>`;
}

export async function renderCapabilityDetail(route) {
  const id = route?.params?.id;
  const root = window.document.getElementById("view");
  if (!root || !id) {
    if (root) root.innerHTML = `<p class="panel p-6 text-center text-[.78rem]">Missing capability id.</p>`;
    return "<p class=\"panel p-6 text-center text-[.78rem]\">Missing capability id.</p>";
  }
  root.innerHTML = renderSkeleton();

  const [capRes, contractRes, reputationRes, versionsRes, datasetsRes, preconditionsRes, latestRes] = await Promise.all([
    api.getCapability(id),
    api.getCapabilityContract(id),
    api.getCapabilityReputation(id),
    api.listVersions(id).catch((e) => ({ ok: false, error: String(e?.message || e) })),
    api.listDatasets(id).catch((e) => ({ ok: false, error: String(e?.message || e) })),
    api.listPreconditions(id).catch((e) => ({ ok: false, error: String(e?.message || e) })),
    api.getLatestVersion(id).catch(() => ({ ok: false })),
  ]);
  if (!capRes.ok) {
    const fallback = `<div class="panel p-6">${errorState(capRes)}</div>`;
    root.innerHTML = fallback;
    return fallback;
  }
  const capability = capRes.data;
  const contract = contractRes.ok ? contractRes.data : null;
  const reputation = reputationRes.ok ? reputationRes.data : null;
  const versions = versionsRes.ok ? versionsRes.data || [] : [];
  const datasets = datasetsRes.ok ? datasetsRes.data || [] : [];
  const preconditions = preconditionsRes.ok ? preconditionsRes.data || [] : [];
  const latestVersion = latestRes.ok ? latestRes.data?.version : null;

  const html = `
    ${header(capability)}
    <section class="mt-5 grid gap-5 xl:grid-cols-[minmax(0,2fr)_minmax(280px,.85fr)]">
      <div class="space-y-5">${versionsCard(versions, latestVersion)}${datasetsCard(datasets)}${preconditionsCard(preconditions)}${contractCard(contract)}<div id="diff-slot"></div></div>
      <div class="space-y-5">${reputationCard(reputation)}${selfEvolveCard(capability.self_evolve)}</div>
    </section>`;
  root.innerHTML = html;
  attach(id, capability, versions, datasets, preconditions);
  return html;
}

function attach(id, capability, versions, datasets, preconditions) {
  const root = window.document.getElementById("view");
  const newVersionBtn = root.querySelector("[data-action=new-version]");
  newVersionBtn?.addEventListener("click", async () => {
    const { openNewVersionModal } = await import("./new-version-modal.js");
    const modalRoot = window.document.getElementById("modal-root");
    await openNewVersionModal(modalRoot, capability, versions);
  });
  root.querySelector("[data-action=edit-contract]")?.addEventListener("click", async () => {
    const { openEditContractModal } = await import("./edit-contract-modal.js");
    const modalRoot = window.document.getElementById("modal-root");
    await openEditContractModal(modalRoot, capability);
  });
  root.querySelector("[data-action=edit-self-evolve]")?.addEventListener("click", async () => {
    const { openEditSelfEvolveModal } = await import("./edit-self-evolve-modal.js");
    const modalRoot = window.document.getElementById("modal-root");
    await openEditSelfEvolveModal(modalRoot, capability);
  });
  root.querySelector("[data-action=edit-capability]")?.addEventListener("click", async () => {
    const { openEditCapabilityModal } = await import("./edit-capability-modal.js");
    const modalRoot = window.document.getElementById("modal-root");
    await openEditCapabilityModal(modalRoot, capability);
  });
  root.querySelector("[data-action=delete-capability]")?.addEventListener("click", async () => {
    if (!window.confirm(`Delete ${capability.name}? This cannot be undone.`)) return;
    const result = await api.deleteCapability(capability.id);
    if (!result.ok) {
      window.alert(`Delete failed: ${apiStatusLabel(result)}`);
      return;
    }
    window.location.hash = "#/capabilities";
    window.location.reload();
  });

  root.querySelectorAll("[data-pick-diff]").forEach((btn) => {
    btn.addEventListener("click", async (event) => {
      const fromVersion = Number(btn.dataset.version);
      const toVersion = Number(window.prompt(`Diff against which version? (existing: ${versions.map((v) => v.version).join(", ")})`, fromVersion === 1 ? versions[versions.length - 1]?.version || 2 : fromVersion - 1));
      if (!toVersion || toVersion === fromVersion) return;
      const slot = root.querySelector("#diff-slot");
      slot.innerHTML = `<div class="panel p-5 sm:p-6"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-3 h-24 w-full"></div></div>`;
      const diff = await api.getCapabilityDiff(id, fromVersion, toVersion);
      slot.innerHTML = diffCard(fromVersion, toVersion, diff.ok ? diff.data : null);
      slot.querySelector("#diff-clear")?.addEventListener("click", () => { slot.innerHTML = ""; });
      event.stopImmediatePropagation();
    });
  });

  root.querySelector("[data-action=new-dataset]")?.addEventListener("click", () => openDatasetCreateModal(capability));
  root.querySelectorAll("[data-dataset-delete]").forEach((b) => b.addEventListener("click", async () => {
    const id = b.dataset.datasetDelete;
    const name = b.dataset.datasetName || id;
    if (!window.confirm(`Delete dataset "${name}"?`)) return;
    const result = await api.deleteDataset(id);
    if (!result.ok) { window.alert(`Delete failed: ${apiStatusLabel(result)}`); return; }
    window.location.reload();
  }));
  root.querySelectorAll("[data-dataset-cases]").forEach((b) => b.addEventListener("click", () => openDatasetCasesModal(b.dataset.datasetCases)));

  root.querySelector("[data-action=new-precondition]")?.addEventListener("click", () => openPreconditionCreateModal(capability));
  root.querySelectorAll("[data-precondition-edit]").forEach((b) => b.addEventListener("click", () => openPreconditionEditModal(b.dataset.preconditionEdit)));
  root.querySelectorAll("[data-precondition-delete]").forEach((b) => b.addEventListener("click", async () => {
    const id = b.dataset.preconditionDelete;
    const name = b.dataset.preconditionName || id;
    if (!window.confirm(`Delete precondition "${name}"?`)) return;
    const result = await api.deletePrecondition(id);
    if (!result.ok) { window.alert(`Delete failed: ${apiStatusLabel(result)}`); return; }
    window.location.reload();
  }));
}

function openDatasetCreateModal(capability) {
  return import("./harness-modals.js").then(({ openDatasetCreateModal: open }) => {
    const modalRoot = document.getElementById("modal-root");
    if (modalRoot) open(modalRoot, capability);
  });
}
function openDatasetCasesModal(datasetId) {
  return import("./harness-modals.js").then(({ openDatasetCasesModal: open }) => {
    const modalRoot = document.getElementById("modal-root");
    if (modalRoot) open(modalRoot, datasetId);
  });
}
function openPreconditionCreateModal(capability) {
  return import("./harness-modals.js").then(({ openPreconditionCreateModal: open }) => {
    const modalRoot = document.getElementById("modal-root");
    if (modalRoot) open(modalRoot, capability);
  });
}
function openPreconditionEditModal(preconditionId) {
  return import("./harness-modals.js").then(({ openPreconditionEditModal: open }) => {
    const modalRoot = document.getElementById("modal-root");
    if (modalRoot) open(modalRoot, preconditionId);
  });
}
