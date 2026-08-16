// Workspace detail page. Shows the workspace's metadata (name,
// organization, timestamps) and a list of its projects. Edit + delete
// actions open inline forms; the delete button asks for confirmation
// before firing the DELETE /api/v1/workspaces/{id} request.
//
// Refactored to consume ui.js primitives — pageHeader, panel, status
// pill, dataTable, inlineBanner — so the workspace detail page
// matches the rest of the dashboard.

import * as api from "../api.js";
import { escape, formatRelative, apiStatusLabel } from "../utils.js";
import { go, currentRoute } from "../router.js";
import { openProjectCreateModal } from "./project-create-modal.js";
import { statusPill, pageHeader, panel, dataTable, emptyState, errorState, inlineBanner } from "../ui.js";

export async function renderWorkspaceDetail(route) {
  const root = window.document.getElementById("view");
  const id = route?.params?.id;
  if (!root) return "";
  if (!id) {
    root.innerHTML = `<div class="mt-5">${errorState({ ok: false, error: "No workspace id in the URL." })}</div>`;
    return;
  }
  // Skeleton while we fetch.
  root.innerHTML = `<section class="panel p-5"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
  const [wsRes, projectsRes] = await Promise.all([
    api.getWorkspace(id),
    api.listProjects(id),
  ]);
  if (!wsRes.ok) {
    root.innerHTML = `<div class="mt-5">${errorState({ ...wsRes, error: `Could not load workspace: ${apiStatusLabel(wsRes)}` })}</div>`;
    return;
  }
  const ws = wsRes.data;
  const projects = projectsRes.ok ? (projectsRes.data || []) : [];
  root.innerHTML = render(ws, projects);
  attach(ws, projects);
}

function render(ws, projects) {
  const breadcrumbs = `<nav class="eyebrow flex items-center gap-2" aria-label="Breadcrumb">
    <a href="#/capabilities" class="text-muted hover:text-ink">Capabilities</a>
    <span class="text-[#b2b3af]">/</span>
    <span>Workspace</span>
  </nav>`;
  return `${pageHeader({
    eyebrow: breadcrumbs,
    title: ws.name,
    description: ws.id,
    actions: `
      <button data-edit-ws class="quiet-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-edit"/></svg>Edit</button>
      <button data-delete-ws class="danger-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-trash"/></svg>Delete workspace</button>
    `,
  })}
  <section class="mt-5 space-y-5">
    ${panel({
      eyebrow: "Workspace metadata",
      title: `${projects.length} project${projects.length === 1 ? "" : "s"}`,
      body: `
        <dl class="grid gap-3 sm:grid-cols-3">
          <div><dt class="eyebrow">Organization</dt><dd class="mt-1 text-[.78rem] font-bold">${escape(ws.organization || "—")}</dd></div>
          <div><dt class="eyebrow">Created</dt><dd class="mt-1 text-[.78rem] font-bold">${escape(formatRelative(ws.created_at))}</dd></div>
          <div><dt class="eyebrow">Updated</dt><dd class="mt-1 text-[.78rem] font-bold">${escape(formatRelative(ws.updated_at))}</dd></div>
        </dl>
        <div data-edit-form class="hidden mt-4 rounded-xl bg-paper p-4"></div>
      `,
    })}
    ${panel({
      eyebrow: "Projects",
      title: "In this workspace",
      rightSlot: `<button data-new-project class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg> New project</button>`,
      body: `<div data-project-list class="mt-2">${renderProjectList(projects, ws.id)}</div>`,
    })}
  </section>`;
}

function renderProjectList(projects, workspaceId) {
  if (!projects.length) {
    return emptyState("No projects in this workspace yet. Create one to start adding capabilities.", { icon: "icon-layers" });
  }
  return dataTable({
    columns: [
      { key: "name", label: "Name", render: (p) => `<a href="#/projects/${escape(p.id)}" class="font-bold hover:underline">${escape(p.name)}</a><span class="block text-[.62rem] text-muted mono">${escape(p.id)}</span>` },
      { key: "description", label: "Description", render: (p) => `<span class="text-muted">${escape(p.description || "—")}</span>` },
      { key: "created", label: "Created", render: (p) => `<span class="mono text-muted">${escape(formatRelative(p.created_at))}</span>` },
      { key: "actions", label: "", align: "right", render: (p) => `<button data-project-delete="${escape(p.id)}" data-project-name="${escape(p.name)}" class="danger-button !text-[.6rem]">Delete</button>` },
    ],
    rows: projects,
    emptyMessage: "No projects yet.",
    emptyIcon: "icon-layers",
  });
}

function renderEditForm(ws) {
  return `<form data-edit-ws-form class="space-y-3">
    <p class="text-[.7rem] text-muted">Update the workspace name and organization. The id is immutable.</p>
    <div><label class="field-label" for="ws-edit-name">Name</label><input id="ws-edit-name" name="name" class="field" required value="${escape(ws.name)}" /></div>
    <div><label class="field-label" for="ws-edit-org">Organization</label><input id="ws-edit-org" name="organization" class="field" value="${escape(ws.organization || "")}" /></div>
    <p data-edit-error class="hidden"></p>
    <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
      <button type="button" class="quiet-button" data-cancel-edit>Cancel</button>
      <button type="submit" class="primary-button">Save</button>
    </div>
  </form>`;
}

function attach(ws, projects) {
  const root = window.document.getElementById("view");
  root.querySelector("[data-new-project]")?.addEventListener("click", () => {
    const modalRoot = document.getElementById("modal-root");
    if (!modalRoot) return;
    openProjectCreateModal(modalRoot, {
      workspaces: [ws],
      preselectWorkspaceId: ws.id,
      onCreated: () => go(currentRoute().path),
    });
  });
  root.querySelector("[data-edit-ws]")?.addEventListener("click", () => {
    const slot = root.querySelector("[data-edit-form]");
    slot.innerHTML = renderEditForm(ws);
    slot.classList.remove("hidden");
    slot.querySelector("[data-cancel-edit]")?.addEventListener("click", () => {
      slot.classList.add("hidden");
    });
    slot.querySelector("[data-edit-ws-form]")?.addEventListener("submit", async (event) => {
      event.preventDefault();
      const data = new FormData(event.target);
      const name = String(data.get("name") || "").trim();
      const organization = String(data.get("organization") || "").trim();
      const errSlot = slot.querySelector("[data-edit-error]");
      errSlot.classList.add("hidden");
      errSlot.innerHTML = "";
      if (!name) {
        errSlot.innerHTML = inlineBanner({ tone: "danger", message: "Name is required." });
        errSlot.classList.remove("hidden");
        return;
      }
      const result = await api.updateWorkspace(ws.id, { name, organization: organization || null });
      if (!result.ok) {
        errSlot.innerHTML = inlineBanner({ tone: "danger", message: `Update failed: ${apiStatusLabel(result)}` });
        errSlot.classList.remove("hidden");
        return;
      }
      go(currentRoute().path);
    });
  });
  root.querySelector("[data-delete-ws]")?.addEventListener("click", async () => {
    if (!window.confirm(`Delete workspace "${ws.name}"? This will also remove all projects, capabilities, releases, and audit history for this workspace. This cannot be undone.`)) return;
    const result = await api.deleteWorkspace(ws.id);
    if (!result.ok) {
      window.alert(`Delete failed: ${apiStatusLabel(result)}`);
      return;
    }
    window.location.hash = "#/capabilities";
    window.location.reload();
  });

  root.querySelectorAll("[data-project-delete]").forEach((b) => b.addEventListener("click", async () => {
    const id = b.dataset.projectDelete;
    const name = b.dataset.projectName || id;
    if (!window.confirm(`Delete project "${name}"?`)) return;
    const result = await api.deleteProject(id);
    if (!result.ok) { window.alert(`Delete failed: ${apiStatusLabel(result)}`); return; }
    window.location.reload();
  }));
}
