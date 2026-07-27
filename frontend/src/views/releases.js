import * as api from "../api.js";
import { escape, formatRelative, apiStatusLabel } from "../utils.js";

const STATUS_TONES = { active: "good", approved: "good", pending: "warn", superseded: "neutral", rolled_back: "danger" };
const ENVS = ["prod", "staging", "dev"];

function pill(text, tone = "neutral") {
  return `<span class="status-pill ${tone} !px-2 !py-1"><span class="status-dot"></span>${escape(text)}</span>`;
}

function rowShell() {
  return `<tr><td colspan="6" class="py-4"><div class="skeleton h-3 w-full"></div></td></tr>`;
}

function renderRow(release, capabilityName) {
  const tone = STATUS_TONES[release.status] || "neutral";
  const envTone = release.environment === "prod" ? "good" : release.environment === "staging" ? "warn" : "neutral";
  return `<tr class="border-t border-line/60 cursor-pointer hover:bg-paper/40" data-open-release="${escape(release.id)}">
    <td class="py-3 pr-3 text-[.66rem] text-muted whitespace-nowrap">${escape(formatRelative(release.created_at))}</td>
    <td class="py-3 pr-3 text-[.7rem] font-bold whitespace-nowrap">${escape(capabilityName)}</td>
    <td class="py-3 pr-3 mono text-[.66rem]">v${escape(release.capability_version)}</td>
    <td class="py-3 pr-3">${pill(release.environment, envTone)}</td>
    <td class="py-3 pr-3">${pill(release.status, tone)}</td>
    <td class="py-3 pr-3 text-[.62rem] text-muted mono truncate max-w-[12rem]" title="${escape(release.id)}">${escape(release.id)}</td>
  </tr>`;
}

function chips(legend, active, dataAttr) {
  return legend.map((item) => {
    const on = (active || "all") === item.value;
    return `<button type="button" data-${dataAttr}="${escape(item.value)}" class="rounded-md ${on ? "bg-ink text-paper" : "bg-paper text-muted hover:text-ink"} px-2.5 py-1.5 text-[.66rem] font-${on ? "bold" : "semibold"}">${escape(item.label)}</button>`;
  }).join("");
}

export async function renderReleases(route) {
  const root = window.document.getElementById("view");
  if (!root) return "";
  const envFilter = route?.query?.env || "all";
  const statusFilter = route?.query?.status || "all";
  root.innerHTML = `<section class="panel p-6"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-10 w-full"></div></section>`;

  const workspaces = await api.listWorkspaces();
  if (!workspaces.ok) {
    root.innerHTML = `<p class="panel p-6 text-center text-[.78rem]">${escape(apiStatusLabel(workspaces))}</p>`;
    return "";
  }
  const allProjects = [];
  const allCaps = [];
  const allRels = [];
  for (const ws of workspaces.data || []) {
    const projects = await api.listProjects(ws.id);
    if (projects.ok) for (const p of projects.data || []) allProjects.push(p);
    await new Promise((r) => setTimeout(r, 60));
  }
  for (const p of allProjects) {
    const caps = await api.listCapabilities(p.id);
    if (caps.ok) for (const c of caps.data || []) allCaps.push(c);
    await new Promise((r) => setTimeout(r, 60));
  }
  for (const c of allCaps) {
    const rels = await api.listReleases(c.id);
    if (rels.ok) for (const r of rels.data || []) allRels.push(r);
    await new Promise((r) => setTimeout(r, 60));
  }
  const capMap = new Map(allCaps.map((c) => [c.id, c]));
  const filtered = allRels.filter((r) =>
    (envFilter === "all" || r.environment === envFilter) &&
    (statusFilter === "all" || r.status === statusFilter)
  ).sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());

  const envChips = chips([{ value: "all", label: "all envs" }, ...ENVS.map((e) => ({ value: e, label: e }))], envFilter, "release-env");
  const statusChips = chips([{ value: "all", label: "all" }, { value: "active", label: "active" }, { value: "approved", label: "approved" }, { value: "pending", label: "pending" }, { value: "superseded", label: "superseded" }, { value: "rolled_back", label: "rolled back" }], statusFilter, "release-status");

  const shell = `
    <section class="flex flex-wrap items-end justify-between gap-3">
      <div>
        <div class="eyebrow">Release pipeline</div>
        <h1 class="mt-2 text-[1.4rem] font-bold tracking-[-.04em]">Releases (${allRels.length} total · ${filtered.length} shown)</h1>
        <p class="mt-1 text-[.78rem] text-muted">Cross-workspace view of every release. Click a row to vote or rollback in the modal.</p>
      </div>
    </section>
    <section class="panel p-5 sm:p-6 mt-5">
      <div class="flex flex-wrap items-end gap-3">
        <div><div class="eyebrow mb-2">Environment</div><div class="flex items-center gap-1 rounded-lg bg-paper p-1" data-release-env-chips>${envChips}</div></div>
        <div><div class="eyebrow mb-2">Status</div><div class="flex items-center gap-1 rounded-lg bg-paper p-1" data-release-status-chips>${statusChips}</div></div>
      </div>
    </section>
    <section class="panel overflow-hidden mt-5">
      <div class="overflow-x-auto">
        <table class="w-full text-[.7rem]">
          <thead><tr class="bg-paper text-left text-[.6rem] uppercase tracking-wider text-muted"><th class="px-4 py-2 font-bold">When</th><th class="px-4 py-2 font-bold">Capability</th><th class="px-4 py-2 font-bold">Version</th><th class="px-4 py-2 font-bold">Environment</th><th class="px-4 py-2 font-bold">Status</th><th class="px-4 py-2 font-bold">Release id</th></tr></thead>
          <tbody id="releases-tbody">${filtered.length ? filtered.map((r) => renderRow(r, capMap.get(r.capability_id)?.name)).join("") : `<tr><td colspan="6" class="py-8 text-center text-[.72rem] text-muted">No releases match this filter.</td></tr>`}</tbody>
        </table>
      </div>
    </section>
  `;
  root.innerHTML = shell;

  root.querySelectorAll("[data-release-env]").forEach((b) => {
    b.addEventListener("click", () => {
      window.location.hash = `#/releases?env=${encodeURIComponent(b.dataset.releaseEnv)}&status=${encodeURIComponent(statusFilter)}`;
      window.location.reload();
    });
  });
  root.querySelectorAll("[data-release-status]").forEach((b) => {
    b.addEventListener("click", () => {
      window.location.hash = `#/releases?env=${encodeURIComponent(envFilter)}&status=${encodeURIComponent(b.dataset.releaseStatus)}`;
      window.location.reload();
    });
  });

  root.querySelector("tbody")?.addEventListener("click", async (event) => {
    const tr = event.target.closest("[data-open-release]");
    if (!tr) return;
    const id = tr.dataset.openRelease;
    const { openReleaseModal } = await import("./release-modal.js");
    const modalRoot = window.document.getElementById("modal-root");
    await openReleaseModal(modalRoot, id);
  });

  return shell;
}
