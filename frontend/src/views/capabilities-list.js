// src/views/capabilities-list.js — cross-workspace capability catalog.
//
// Lists every project that owns at least one capability, groups the
// rows under each project, and offers an in-place filter. The page
// uses ui.js primitives so the header, panels, status pills, and
// empty states match the rest of the dashboard.

import * as api from "../api.js";
import { escape } from "../utils.js";
import { ownerName } from "../state/owners.js";
import { openWorkspaceCreateModal } from "./workspace-create-modal.js";
import { openProjectCreateModal } from "./project-create-modal.js";
import { statusPill, pageHeader, panel, emptyState, errorState } from "../ui.js";

const STATUS_LABELS = { active: "Production", approved: "Approved", pending: "Pending", superseded: "Superseded", rolled_back: "Rolled back" };
const ENV_TONES = { prod: "good", staging: "warn", dev: "neutral" };
function envTone(env) { return ENV_TONES[env] || "neutral"; }
function statusLabel(s) { return STATUS_LABELS[s] || "Draft"; }

function pickRelease(releases) {
  return releases.find((r) => r.status === "active")
      || releases.find((r) => r.status === "pending")
      || releases.find((r) => r.status === "approved")
      || releases[0]
      || null;
}

function rowContent(cap, release, version) {
  const statusText = statusLabel(release?.status);
  const statusTone = release ? envTone(release.environment) : "neutral";
  const envLabel = release ? release.environment : "—";
  return `<a href="#/capabilities/${escape(cap.id)}" data-searchable class="data-row grid gap-3 px-5 py-4 md:grid-cols-[minmax(220px,1.7fr)_120px_140px_120px_140px_32px] md:items-center md:gap-4">
    <div class="flex min-w-0 items-center gap-3">
      <span class="grid h-9 w-9 shrink-0 place-items-center rounded-[10px] bg-[#e9ecff] text-blue"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-spark"/></svg></span>
      <span class="min-w-0"><span class="block truncate text-[.78rem] font-bold">${escape(cap.name)}</span><span class="mt-1 block truncate text-[.64rem] text-muted">${escape(cap.description || (cap.tags && cap.tags.length ? cap.tags.join(" · ") : "No description"))}</span></span>
    </div>
    <span class="mono text-[.68rem] text-[#62656a]">v${escape(version)}</span>
    <span>${statusPill(envLabel, statusTone)}</span>
    <span>${statusPill(statusText, statusText === "Draft" ? "neutral" : "good")}</span>
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
        <h3 class="text-[.84rem] font-bold">
          <a href="#/workspaces/${escape(project.workspace_id || "")}" class="hover:underline">${escape(project.workspaceName || "workspace")}</a>
          <span class="text-[#b2b3af]">/</span>
          <a href="#/projects/${escape(project.id)}" class="hover:underline">${escape(project.name)}</a>
        </h3>
        <span class="mt-0.5 block text-[.62rem] text-muted">${escape(project.id.slice(-8))} · ${capabilities.length} capabilit${capabilities.length === 1 ? "y" : "ies"}</span>
      </div>
      <button data-new-capability-for-project="${escape(project.id)}" class="quiet-button !h-8 !text-[.66rem]"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>Add capability</button>
    </header>
    <div class="divide-y divide-line/60">${rows || emptyState("No capabilities yet — use the button above to add the first one.", { icon: "icon-layers" })}</div>
  </section>`;
}

export async function renderCapabilitiesList(route) {
  const root = window.document.getElementById("view");
  if (!root) return "";

  const shell = `${pageHeader({
    eyebrow: "Catalog",
    title: "Capabilities",
    description: "Cross-project view of every capability. Grouped by project, click any row to manage versions, contract, self-evolve, and reputation.",
    actions: `
      <input id="cap-search" placeholder="Filter by name, project, owner" class="field !h-9 !w-72 !rounded-lg !border-line !bg-white/65 !text-[.72rem]" aria-label="Filter capabilities" />
      <button data-new-capability class="primary-button !h-9"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg> New capability</button>
    `,
  })}
  <section id="cap-list" class="mt-5 space-y-5"></section>`;
  root.innerHTML = shell;

  const workspacesRes = await api.listWorkspaces();
  const workspaces = workspacesRes.ok ? (workspacesRes.data || []) : [];
  const projectsRes = await (async () => {
    const allProjects = [];
    for (const w of workspaces) {
      const p = await api.listProjects(w.id);
      if (p.ok) for (const proj of p.data || []) allProjects.push({ ...proj, workspaceName: w.name });
      await new Promise((r) => setTimeout(r, 40));
    }
    return { ok: true, data: allProjects };
  })();
  const list = root.querySelector("#cap-list");
  if (!projectsRes.ok) {
    list.innerHTML = errorState(projectsRes);
    return shell;
  }
  const projects = projectsRes.data || [];
  if (projects.length === 0) {
    if (workspaces.length === 0) {
      list.innerHTML = emptyStateWithAction("No workspaces yet", "Workspaces hold projects, which hold capabilities.", "Create workspace", "data-new-workspace", "icon-grid");
    } else {
      list.innerHTML = emptyStateWithAction("No projects yet", "Pick a workspace and create your first project.", "Create project", "data-new-project", "icon-layers");
    }
    list.querySelector("[data-new-workspace]")?.addEventListener("click", () => {
      const modalRoot = document.getElementById("modal-root") || root;
      openWorkspaceCreateModal(modalRoot, { onCreated: () => render(route) });
    });
    list.querySelector("[data-new-project]")?.addEventListener("click", () => {
      const modalRoot = document.getElementById("modal-root") || root;
      openProjectCreateModal(modalRoot, { workspaces, onCreated: () => render(route) });
    });
    root.querySelector("[data-new-capability]")?.addEventListener("click", () => {
      const modalRoot = document.getElementById("modal-root") || root;
      if (workspaces.length === 0) openWorkspaceCreateModal(modalRoot, { onCreated: () => render(route) });
      else openProjectCreateModal(modalRoot, { workspaces, onCreated: () => render(route) });
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
  if (summary) summary.textContent = `(${allCaps.length} across ${projects.length} project${projects.length === 1 ? "" : "s"})`;

  list.innerHTML = projects.map((project) => {
    const caps = capsByProject.get(project.id) || [];
    return projectSection(project, caps, releasesByCap);
  }).join("") || emptyState("No capabilities yet.", { icon: "icon-layers" });

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
  return { projects: projectsRes || [], preselectId: preselectId || null };
}

function emptyStateWithAction(title, message, actionLabel, dataAttr, icon) {
  return `<div class="empty-state">
    <span class="empty-state-icon"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#${icon}"/></svg></span>
    <p class="font-bold text-ink">${escape(title)}</p>
    <p class="mt-1">${escape(message)}</p>
    <button ${dataAttr} class="primary-button mt-4"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg> ${escape(actionLabel)}</button>
  </div>`;
}
