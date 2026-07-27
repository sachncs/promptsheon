import * as api from "../api.js";
import { escape, formatRelative } from "../utils.js";
import { ownerName } from "../state/owners.js";

const STATUS_TONES = { active: "good", approved: "good", pending: "warn", superseded: "neutral", rolled_back: "danger" };
const ENV_TONES = { prod: "good", staging: "warn", dev: "neutral" };

function pill(text, tone = "neutral") {
  return `<span class="status-pill ${tone} !px-2 !py-1"><span class="status-dot"></span>${escape(text)}</span>`;
}

function pickRelease(releases) {
  return releases.find((r) => r.status === "active")
      || releases.find((r) => r.status === "pending")
      || releases.find((r) => r.status === "approved")
      || releases[0]
      || null;
}

function rowContent(cap, release, version) {
  const statusLabel = !release ? "Draft" : ({ active: "Production", approved: "Approved", pending: "Pending", superseded: "Superseded", rolled_back: "Rolled back" })[release.status] || "Draft";
  const statusTone = release ? (ENV_TONES[release.environment] || "neutral") : "neutral";
  const envLabel = release ? release.environment : "—";
  return `<a href="#/capabilities/${escape(cap.id)}" data-searchable class="data-row grid gap-3 px-5 py-4 md:grid-cols-[minmax(220px,1.7fr)_120px_140px_120px_140px_32px] md:items-center md:gap-4">
    <div class="flex min-w-0 items-center gap-3">
      <span class="grid h-9 w-9 shrink-0 place-items-center rounded-[10px] bg-[#e9ecff] text-blue"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-spark"/></svg></span>
      <span class="min-w-0"><span class="block truncate text-[.78rem] font-bold">${escape(cap.name)}</span><span class="mt-1 block truncate text-[.64rem] text-muted">${escape(cap.description || (cap.tags && cap.tags.length ? cap.tags.join(" · ") : "No description"))}</span></span>
    </div>
    <span class="mono text-[.68rem] text-[#62656a]">v${escape(version)}</span>
    <span>${pill(envLabel, statusTone)}</span>
    <span>${pill(statusLabel, statusLabel === "Draft" ? "neutral" : "good")}</span>
    <span class="text-[.72rem] text-[#686b70]">${escape(ownerName(cap.owner))}</span>
    <span class="icon-button !h-7 !w-7 !border-0 !bg-transparent"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.8]"><use href="#icon-arrow-right"/></svg></span>
  </a>`;
}

function projectSection(project, capabilities, releasesByCap) {
  const rows = capabilities.map((cap) => {
    const releases = releasesByCap.get(cap.id) || [];
    const release = pickRelease(releases);
    const version = releases.reduce((m, r) => Math.max(m, r.capability_version || 0), 0) || 1;
    return rowContent(cap, release, version);
  }).join("");
  return `<section class="panel overflow-hidden">
    <header class="flex items-center justify-between border-b border-line/70 bg-paper/60 px-5 py-3">
      <div>
        <h3 class="text-[.84rem] font-bold">${escape(project.name)}</h3>
        <span class="mt-0.5 block text-[.62rem] text-muted">${escape(project.id.slice(-8))} · ${capabilities.length} capabilit${capabilities.length === 1 ? "y" : "ies"}</span>
      </div>
      <button data-new-capability-for-project="${escape(project.id)}" class="quiet-button !h-8 !text-[.66rem]"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>Add capability</button>
    </header>
    <div class="divide-y divide-line/60">${rows || `<p class="px-5 py-6 text-center text-[.66rem] text-muted">No capabilities yet — use the button above to add the first one.</p>`}</div>
  </section>`;
}

export async function renderCapabilitiesList(route) {
  const root = window.document.getElementById("view");
  if (!root) return "";

  const shell = `
    <section class="flex flex-wrap items-end justify-between gap-3">
      <div>
        <div class="eyebrow">Catalog</div>
        <h1 class="mt-2 text-[1.4rem] font-bold tracking-[-.04em]">Capabilities <span id="cap-summary" class="text-muted font-normal">—</span></h1>
        <p class="mt-1 text-[.78rem] text-muted">Cross-project view of every capability. Grouped by project, click any row to manage versions, contract, self-evolve, and reputation.</p>
      </div>
      <div class="flex items-center gap-2">
        <input id="cap-search" placeholder="Filter by name, project, owner" class="field !h-9 !w-72 !rounded-lg !border-line !bg-white/65 !text-[.72rem]" />
        <button data-new-capability class="primary-button !h-9"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg> New capability</button>
      </div>
    </section>
    <section id="cap-list" class="mt-5 space-y-5"></section>
  `;
  root.innerHTML = shell;

  const projectsRes = await api.listWorkspaces().then(async (ws) => {
    if (!ws.ok) return ws;
    const workspaces = ws.data || [];
    const allProjects = [];
    for (const w of workspaces) {
      const p = await api.listProjects(w.id);
      if (p.ok) for (const proj of p.data || []) allProjects.push({ ...proj, workspaceName: w.name });
      await new Promise((r) => setTimeout(r, 40));
    }
    return { ok: true, data: allProjects };
  });
  const list = root.querySelector("#cap-list");
  if (!projectsRes.ok) {
    list.innerHTML = `<p class="panel p-5 text-center text-[.78rem] text-muted">${escape(projectsRes.error || "Failed to load catalog.")}</p>`;
    return shell;
  }
  const projects = projectsRes.data || [];
  if (projects.length === 0) {
    list.innerHTML = `<div class="rounded-xl border border-dashed border-line bg-paper p-8 text-center"><p class="text-[.78rem] font-bold">No projects yet</p><p class="mt-1 text-[.66rem] text-muted">Create a project in your workspace to host capabilities.</p></div>`;
    root.querySelector("[data-new-capability]")?.addEventListener("click", () => {
      window.alert("Create a project first via the API: POST /api/v1/workspaces/{id}/projects");
    });
    return shell;
  }

  const allCaps = [];
  const releasesByCap = new Map();
  for (const project of projects) {
    const caps = await api.listCapabilities(project.id);
    if (caps.ok) for (const c of caps.data || []) allCaps.push({ ...c, projectId: project.id });
    await new Promise((r) => setTimeout(r, 40));
  }
  for (const cap of allCaps) {
    const rels = await api.listReleases(cap.id);
    if (rels.ok) releasesByCap.set(cap.id, rels.data || []);
    await new Promise((r) => setTimeout(r, 40));
  }

  const capsByProject = new Map();
  for (const cap of allCaps) {
    if (!capsByProject.has(cap.projectId)) capsByProject.set(cap.projectId, []);
    capsByProject.get(cap.projectId).push(cap);
  }

  const summary = root.querySelector("#cap-summary");
  summary.textContent = `(${allCaps.length} across ${projects.length} project${projects.length === 1 ? "" : "s"})`;

  list.innerHTML = projects.map((project) => {
    const caps = capsByProject.get(project.id) || [];
    return projectSection(project, caps, releasesByCap);
  }).join("") || `<p class="panel p-5 text-center text-[.78rem] text-muted">No capabilities yet.</p>`;

  attach(root);
  return shell;
}

function attach(root) {
  const search = root.querySelector("#cap-search");
  search?.addEventListener("input", () => {
    const q = search.value.trim().toLowerCase();
    root.querySelectorAll("[data-searchable]").forEach((row) => {
      row.hidden = q.length > 0 && !row.textContent.toLowerCase().includes(q);
    });
  });
  root.querySelector("[data-new-capability]")?.addEventListener("click", async () => {
    const modalRoot = document.getElementById("modal-root");
    const { openNewCapabilityModal } = await import("./new-capability-modal.js");
    if (modalRoot) await openNewCapabilityModal(modalRoot, await loadProjectsForModal());
  });
  root.querySelectorAll("[data-new-capability-for-project]").forEach((btn) => {
    btn.addEventListener("click", async (event) => {
      event.stopPropagation();
      const projectId = btn.dataset.newCapabilityForProject;
      const modalRoot = document.getElementById("modal-root");
      const { openNewCapabilityModal } = await import("./new-capability-modal.js");
      if (modalRoot) await openNewCapabilityModal(modalRoot, await loadProjectsForModal(projectId));
    });
  });
}

async function loadProjectsForModal(preselectId) {
  const projectsRes = await api.listWorkspaces().then(async (ws) => {
    if (!ws.ok) return [];
    const all = [];
    for (const w of ws.data || []) {
      const p = await api.listProjects(w.id);
      if (p.ok) for (const proj of p.data || []) all.push(proj);
    }
    return all;
  });
  if (preselectId) {
    projectsRes.forEach((p) => p.selected = p.id === preselectId);
  }
  return projectsRes;
}
