import * as api from "../api.js";
import { escape, formatPercent, formatRelative, apiStatusLabel } from "../utils.js";
import { ownerName } from "../state/owners.js";

const skeletons = {
  header: () => `<div class="panel p-6"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-10 w-80"></div><div class="skeleton mt-3 h-4 w-64"></div></div>`,
  versions: () => `<div class="panel p-5 sm:p-6"><div class="skeleton h-3 w-24"></div><div class="skeleton mt-4 h-12 w-full"></div><div class="skeleton mt-4 h-12 w-full"></div></div>`,
  contract: () => `<div class="panel p-5 sm:p-6"><div class="skeleton h-3 w-28"></div><div class="skeleton mt-4 h-8 w-full"></div><div class="skeleton mt-3 h-8 w-full"></div></div>`,
  reputation: () => `<div class="panel p-5"><div class="skeleton h-3 w-24"></div><div class="skeleton mt-4 h-20 w-32"></div></div>`
};

function contractBlock(contract) {
  if (!contract) return `<p class="mt-2 text-[.68rem] text-muted">No contract set. Capabilities without a contract cannot be auto-promoted by the recommendation engine.</p>`;
  const slo = contract.slo_target || {};
  const blast = contract.blast_radius || "—";
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

function optional(label, value) {
  return `<div class="rounded-lg bg-paper p-3"><span class="eyebrow">${escape(label)}</span><span class="mt-2 block text-[.8rem] font-bold break-words">${escape(value)}</span></div>`;
}

function schema(label, schema) {
  return `<details class="mt-3 rounded-lg bg-paper p-3 text-[.66rem]"><summary class="cursor-pointer text-[.72rem] font-bold">${escape(label)}</summary><pre class="mt-2 overflow-x-auto text-[.62rem] mono">${escape(JSON.stringify(schema, null, 2))}</pre></details>`;
}

function renderVersionsTable(versions, currentId, capabilityId) {
  if (!versions?.length) return `<p class="mt-3 text-[.68rem] text-muted">No versions yet. Create the first version to enable releases.</p>`;
  return `<div class="mt-3 overflow-x-auto"><table class="w-full text-[.7rem]"><thead><tr class="text-left text-[.62rem] uppercase tracking-wider text-muted"><th class="py-1.5 font-bold">Version</th><th class="py-1.5 font-bold">Manifest hash</th><th class="py-1.5 font-bold">Created</th><th class="py-1.5 font-bold">By</th><th class="py-1.5 font-bold text-right"></th></tr></thead><tbody>${versions.slice(0, 30).map((v) => `
    <tr class="border-t border-line/60">
      <td class="py-2 mono font-bold"><a href="#/versions/${escape(v.id)}" class="hover:underline">v${escape(v.version)}</a></td>
      <td class="py-2 mono truncate max-w-[12rem]">${escape((v.manifest_hash || "").slice(0, 12))}…</td>
      <td class="py-2 text-muted">${escape(formatRelative(v.created_at))}</td>
      <td class="py-2 text-muted">${escape(v.created_by || "—")}</td>
      <td class="py-2 text-right"><button type="button" data-pick-diff="${escape(v.id)}" data-version="${escape(v.version)}" class="rounded-md bg-paper px-2.5 py-1 text-[.6rem] font-bold text-ink hover:bg-ink hover:text-paper">Diff against v${escape(v.version)}</button></td>
    </tr>`).join("")}</tbody></table></div>
  <p class="mt-3 text-[.62rem] text-muted">Pick any version above to diff it against another version of the same capability.</p>`;
}

function reputationGauge(rep) {
  if (!rep) return `<p class="mt-3 text-[.68rem] text-muted">No reputation data yet (no evaluations have run).</p>`;
  const score = Math.max(0, Math.min(1, rep.trust_score || 0));
  const pct = (score * 100).toFixed(0);
  const r = 28;
  const cx = 32; const cy = 32;
  const dash = 2 * Math.PI * r;
  const filled = dash * score;
  const ring = `<circle cx="${cx}" cy="${cy}" r="${r}" fill="none" stroke="#e6e6e1" stroke-width="8"></circle>
    <circle cx="${cx}" cy="${cy}" r="${r}" fill="none" stroke="url(#gauge)" stroke-width="8" stroke-linecap="round" stroke-dasharray="${filled} ${dash - filled}" transform="rotate(-90 ${cx} ${cy})"></circle>`;
  return `<div class="mt-3 flex items-center gap-4">
    <svg viewBox="0 0 64 64" class="h-20 w-20">
      <defs><linearGradient id="gauge" x1="0" x2="1" y1="0" y2="0"><stop offset="0" stop-color="#789c35"/><stop offset="1" stop-color="#c9f36a"/></linearGradient></defs>
      ${ring}
      <text x="32" y="36" text-anchor="middle" font-size="14" font-weight="700" fill="#151619">${pct}</text>
    </svg>
    <div class="flex-1 space-y-1.5 text-[.7rem]">
      ${ratioRow("Eval pass rate", rep.eval_pass_rate)}
      ${ratioRow("SLO adherence", rep.slo_adherence_rate)}
      ${ratioRow("Decision adoption", rep.decision_adoption_rate)}
      <div class="text-[.62rem] text-muted">Sample size ${escape(rep.sample_size || 0)}</div>
    </div>
  </div>`;
}

function ratioRow(label, value) {
  const pct = typeof value === "number" ? formatPercent(value * 100) : "—";
  const pctNum = typeof value === "number" ? Math.max(0, Math.min(100, value * 100)) : 0;
  return `<div><div class="flex items-center justify-between"><span class="text-[.66rem] text-muted">${escape(label)}</span><span class="mono text-[.66rem] font-bold">${escape(pct)}</span></div><div class="mt-1 h-1.5 overflow-hidden rounded-full bg-paper"><div class="h-full rounded-full bg-[#789c35]" style="width:${pctNum.toFixed(1)}%"></div></div></div>`;
}

function header(c) {
  return `<div class="panel p-6">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0">
        <div class="eyebrow">${escape(c.project_id || "")}</div>
        <h1 class="mt-2 text-[clamp(1.4rem,2.5vw,1.8rem)] font-bold tracking-[-.04em]">${escape(c.name)}</h1>
        <p class="mt-1 text-[.78rem] text-muted">${escape(c.description || "No description yet.")}</p>
        <div class="mt-3 flex flex-wrap items-center gap-2 text-[.66rem] text-muted">
          <span class="status-pill neutral !px-2 !py-1">${escape(c.id.slice(-8))}</span>
          <span>Owner <span class="font-semibold text-ink">${escape(ownerName(c.owner))}</span></span>
          ${c.tags?.length ? `<span>· Tags <span class="font-semibold text-ink">${escape(c.tags.join(", "))}</span></span>` : ""}
          <span>· Created ${escape(formatRelative(c.created_at))}</span>
          <span>· Updated ${escape(formatRelative(c.updated_at))}</span>
        </div>
      </div>
      <div class="flex shrink-0 items-center gap-2">
        <button data-action="edit-capability" class="quiet-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-edit"/></svg>Edit</button>
        <button data-action="delete-capability" class="quiet-button !text-rose-700"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-trash"/></svg>Delete</button>
      </div>
    </div>
  </div>`;
}

function versionsCard(versions, latestVersion) {
  const latestPill = latestVersion != null
    ? `<span class="status-pill good !px-2 !py-1">Latest v${escape(String(latestVersion))}</span>`
    : "";
  return `<div class="panel p-5 sm:p-6">
    <div class="flex items-center justify-between"><div class="flex items-center gap-2"><div class="eyebrow">Versions</div>${latestPill}</div><button data-action="new-version" class="quiet-button !text-[.66rem]"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>New version</button></div>
    ${renderVersionsTable(versions)}
  </div>`;
}

function datasetsCard(datasets) {
  const rows = (datasets || []).map((d) => `<li class="flex items-center justify-between border-t border-line/60 px-5 py-3" data-dataset-id="${escape(d.id)}">
    <div>
      <p class="text-[.78rem] font-bold">${escape(d.name || "—")}</p>
      <p class="text-[.62rem] text-muted mono">${escape(d.id)}</p>
    </div>
    <div class="flex items-center gap-2">
      <button data-dataset-cases="${escape(d.id)}" class="quiet-button !text-[.62rem]">Cases</button>
      <button data-dataset-delete="${escape(d.id)}" data-dataset-name="${escape(d.name || d.id)}" class="rounded-md bg-rose-50 px-2 py-1 text-[.62rem] font-bold text-rose-700 hover:bg-rose-100">Delete</button>
    </div>
  </li>`).join("");
  return `<div class="panel p-5 sm:p-6">
    <div class="flex items-center justify-between"><div class="eyebrow">Datasets</div><button data-action="new-dataset" class="quiet-button !text-[.66rem]"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>New dataset</button></div>
    ${datasets?.length ? `<ul class="mt-3 -mx-5">${rows}</ul>` : `<p class="mt-3 text-[.68rem] text-muted">No datasets yet. Create one to start running eval cases.</p>`}
  </div>`;
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
      <button data-precondition-delete="${escape(p.id)}" data-precondition-name="${escape(p.name || p.id)}" class="rounded-md bg-rose-50 px-2 py-1 text-[.62rem] font-bold text-rose-700 hover:bg-rose-100">Delete</button>
    </div>
  </li>`).join("");
  return `<div class="panel p-5 sm:p-6">
    <div class="flex items-center justify-between"><div class="eyebrow">Preconditions</div><button data-action="new-precondition" class="quiet-button !text-[.66rem]"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>New precondition</button></div>
    ${preconditions?.length ? `<ul class="mt-3 -mx-5">${rows}</ul>` : `<p class="mt-3 text-[.68rem] text-muted">No preconditions yet. Add command hooks that must pass before a release can activate.</p>`}
  </div>`;
}

function contractCard(contract) {
  return `<div class="panel p-5 sm:p-6">
    <div class="flex items-center justify-between"><div class="eyebrow">Capability contract</div><button data-action="edit-contract" class="quiet-button !text-[.66rem]"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-edit"/></svg>Edit</button></div>
    ${contractBlock(contract)}
  </div>`;
}

function selfEvolveCard(config) {
  const enabled = !!config?.enabled;
  return `<div class="panel p-5 sm:p-6">
    <div class="flex items-center justify-between"><div class="eyebrow">Self-evolution</div><span class="status-pill ${enabled ? "good" : "neutral"} !px-2 !py-1">${enabled ? "on" : "off"}</span></div>
    <div class="mt-3 grid gap-3 sm:grid-cols-2">
      ${optional("Min score", config?.min_score ?? "—")}
      ${optional("Max revisions", config?.max_revisions ?? "—")}
      ${optional("Cooldown (s)", config?.cooldown_sec ?? "—")}
      ${optional("Target env", config?.target_env ?? "—")}
    </div>
    <div class="mt-3 flex justify-end"><button data-action="edit-self-evolve" class="quiet-button !text-[.66rem]"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-edit"/></svg>Configure</button></div>
  </div>`;
}

function reputationCard(rep) {
  return `<div class="panel p-5 sm:p-6">
    <div class="eyebrow">Reputation</div>
    ${reputationGauge(rep)}
  </div>`;
}

function diffCard(fromVersion, toVersion, diff) {
  if (!diff) return `<div class="panel p-5 sm:p-6"><div class="eyebrow">Diff ${escape(fromVersion)} → ${escape(toVersion)}</div><p class="mt-2 text-[.68rem] text-muted">Pick two versions above to see added, removed, and changed artifacts.</p></div>`;
  const section = (title, items, tone) => `<div class="mt-3"><div class="eyebrow">${title} (${items.length})</div>${items.length ? `<ul class="mt-2 space-y-1 text-[.7rem] mono">${items.map((it) => `<li class="flex items-center gap-2 rounded-md bg-${tone}-50 px-2 py-1"><span class="font-bold">${escape(it.kind)}</span><span class="truncate text-muted">${escape(JSON.stringify(it.hash || it.old_hash || it.new_hash || ""))}</span></li>`).join("")}</ul>` : `<p class="mt-1 text-[.68rem] text-muted">None.</p>`}</div>`;
  return `<div class="panel p-5 sm:p-6">
    <div class="flex items-center justify-between"><div class="eyebrow">Diff v${escape(fromVersion)} → v${escape(toVersion)}</div><button id="diff-clear" class="quiet-button !text-[.66rem]">Close</button></div>
    ${section("Added", diff.added || [], "lime")}
    ${section("Removed", diff.removed || [], "rose")}
    ${section("Changed", diff.changed || [], "amber")}
  </div>`;
}

function renderSkeleton() {
  return `
    <section class="grid gap-5 xl:grid-cols-[minmax(0,2fr)_minmax(280px,.85fr)]">
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
    api.getLatestVersion(id).catch(() => ({ ok: false }))
  ]);
  if (!capRes.ok) {
    const fallback = `<p class="panel p-6 text-center text-[.78rem]">${escape(apiStatusLabel(capRes))}</p>`;
    root.innerHTML = fallback;
    return fallback;
  }
  const capability = capRes.data;
  const contract = contractRes.ok ? contractRes.data : null;
  const reputation = reputationRes.ok ? reputationRes.data : null;
  const versions = versionsRes.ok ? versionsRes.data || [] : [];
  const datasets = datasetsRes.ok ? datasetsRes.data || [] : [];
  const preconditions = preconditionsRes.ok ? preconditionsRes.data || [] : [];
  // Backend's latest-version endpoint returns {version, ...}; the
  // badge surfaces it alongside the full versions list.
  const latestVersion = latestRes.ok ? latestRes.data?.version : null;

  const html = `
    <section class="grid gap-5 xl:grid-cols-[minmax(0,2fr)_minmax(280px,.85fr)]">
      <div class="space-y-5">${header(capability)}${versionsCard(versions, latestVersion)}${datasetsCard(datasets)}${preconditionsCard(preconditions)}${contractCard(contract)}<div id="diff-slot"></div></div>
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

  // Datasets
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

  // Preconditions
  root.querySelector("[data-action=new-precondition]")?.addEventListener("click", () => openPreconditionCreateModal(capability));
  root.querySelectorAll("[data-precondition-edit]").forEach((b) => b.addEventListener("click", () => openPreconditionEditModal(b.dataset.preconditionEdit, preconditions)));
  root.querySelectorAll("[data-precondition-delete]").forEach((b) => b.addEventListener("click", async () => {
    const id = b.dataset.preconditionDelete;
    const name = b.dataset.preconditionName || id;
    if (!window.confirm(`Delete precondition "${name}"?`)) return;
    const result = await api.deletePrecondition(id);
    if (!result.ok) { window.alert(`Delete failed: ${apiStatusLabel(result)}`); return; }
    window.location.reload();
  }));
}

// Dataset create modal
async function openDatasetCreateModal(capability) {
  const { openDatasetCreateModal: open } = await import("./harness-modals.js");
  await open(window.document.getElementById("modal-root"), capability);
}

// Precondition create modal
async function openPreconditionCreateModal(capability) {
  const { openPreconditionCreateModal: open } = await import("./harness-modals.js");
  await open(window.document.getElementById("modal-root"), capability);
}

// Precondition edit modal
async function openPreconditionEditModal(id, preconditions) {
  const target = preconditions.find((p) => p.id === id) || null;
  const { openPreconditionEditModal: open } = await import("./harness-modals.js");
  await open(window.document.getElementById("modal-root"), id, target);
}

// Dataset cases (bulk-write JSON) modal
async function openDatasetCasesModal(id) {
  const { openDatasetCasesModal: open } = await import("./harness-modals.js");
  await open(window.document.getElementById("modal-root"), id);
}
