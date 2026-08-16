// Project detail page. Shows the project's metadata and the
// capabilities it contains. Edit + delete actions open inline
// forms; the delete button asks for confirmation before firing
// DELETE /api/v1/projects/{id}.
//
// Migrated to ui.js primitives — pageHeader / panel / statusPill /
// dataTable / emptyState / errorState / inlineBanner — so this
// detail view matches workspace-detail + capability-detail.

import * as api from "../api.js";
import { escape, formatRelative, apiStatusLabel } from "../utils.js";
import { go, currentRoute } from "../router.js";
import { openNewCapabilityModal } from "./new-capability-modal.js";
import { statusPill, pageHeader, panel, dataTable, emptyState, errorState, inlineBanner } from "../ui.js";

export async function renderProjectDetail(route) {
  const root = window.document.getElementById("view");
  const id = route?.params?.id;
  if (!root) return "";
  if (!id) {
    root.innerHTML = `<div class="mt-5">${errorState({ ok: false, error: "No project id in the URL." })}</div>`;
    return;
  }
  root.innerHTML = `<section class="panel p-5"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
  const projRes = await api.getProject(id);
  if (!projRes.ok) {
    root.innerHTML = `<div class="mt-5">${errorState({ ...projRes, error: `Could not load project: ${apiStatusLabel(projRes)}` })}</div>`;
    return;
  }
  const project = projRes.data;
  const capsRes = await api.listCapabilities(id);
  const capabilities = capsRes.ok ? (capsRes.data || []) : [];
  let workspace = null;
  if (project.workspace_id) {
    const wsRes = await api.getWorkspace(project.workspace_id);
    if (wsRes.ok) workspace = wsRes.data;
  }
  root.innerHTML = render(project, workspace, capabilities);
  attach(project, workspace, capabilities);
}

function render(project, workspace, capabilities) {
  const wsCrumb = workspace
    ? `<a href="#/workspaces/${escape(workspace.id)}" class="hover:text-ink">${escape(workspace.name)}</a><span class="text-[#b2b3af]">/</span>`
    : "";
  const breadcrumbs = `<nav class="eyebrow flex items-center gap-2" aria-label="Breadcrumb">${wsCrumb}<span>Project</span></nav>`;
  return `${pageHeader({
    eyebrow: breadcrumbs,
    title: project.name,
    description: project.id,
    actions: `
      <button data-edit-proj class="quiet-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-edit"/></svg>Edit</button>
      <button data-delete-proj class="danger-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-trash"/></svg>Delete project</button>
    `,
  })}
  <section class="mt-5 space-y-5">
    ${panel({
      eyebrow: "Project metadata",
      title: `${capabilities.length} capability${capabilities.length === 1 ? "" : "ies"}`,
      body: `
        <dl class="grid gap-3 sm:grid-cols-3">
          <div><dt class="eyebrow">Description</dt><dd class="mt-1 text-[.78rem] font-bold">${escape(project.description || "—")}</dd></div>
          <div><dt class="eyebrow">Workspace</dt><dd class="mt-1 text-[.78rem] font-bold">${workspace ? escape(workspace.name) : (project.workspace_id ? `<a href="#/workspaces/${escape(project.workspace_id)}" class="hover:underline">${escape(project.workspace_id).slice(-8)}</a>` : "—")}</dd></div>
          <div><dt class="eyebrow">Created</dt><dd class="mt-1 text-[.78rem] font-bold">${escape(formatRelative(project.created_at))}</dd></div>
        </dl>
        <p data-edit-form class="hidden mt-4 rounded-xl bg-paper p-4"></p>
      `,
    })}
    ${panel({
      eyebrow: "Capabilities",
      title: "In this project",
      rightSlot: `<button data-new-cap class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg> New capability</button>`,
      body: `<div data-cap-list class="mt-2">${renderCapList(capabilities)}</div>`,
    })}
  </section>`;
}

function renderCapList(capabilities) {
  if (!capabilities.length) {
    return emptyState("No capabilities in this project yet.", { icon: "icon-layers" });
  }
  return dataTable({
    columns: [
      { key: "name", label: "Name", render: (c) => `<a href="#/capabilities/${escape(c.id)}" class="font-bold hover:underline">${escape(c.name)}</a><span class="block text-[.62rem] text-muted mono">${escape(c.id)}</span>` },
      { key: "description", label: "Description", render: (c) => `<span class="text-muted">${escape(c.description || "—")}</span>` },
      { key: "created", label: "Created", render: (c) => `<span class="mono text-muted">${escape(formatRelative(c.created_at))}</span>` },
      { key: "actions", label: "", align: "right", render: (c) => `<button data-cap-delete="${escape(c.id)}" data-cap-name="${escape(c.name)}" class="danger-button !text-[.6rem]">Delete</button>` },
    ],
    rows: capabilities,
    emptyMessage: "No capabilities yet.",
    emptyIcon: "icon-layers",
  });
}

function renderEditForm(project) {
  return `<form data-edit-proj-form class="space-y-3">
    <p class="text-[.7rem] text-muted">Update the project name and description. The id and workspace are immutable.</p>
    <div><label class="field-label" for="proj-edit-name">Name</label><input id="proj-edit-name" name="name" class="field" required value="${escape(project.name)}" /></div>
    <div><label class="field-label" for="proj-edit-desc">Description</label><textarea id="proj-edit-desc" name="description" class="field" rows="3">${escape(project.description || "")}</textarea></div>
    <p data-edit-error class="hidden"></p>
    <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
      <button type="button" class="quiet-button" data-cancel-edit>Cancel</button>
      <button type="submit" class="primary-button">Save</button>
    </div>
  </form>`;
}

function attach(project, workspace, capabilities) {
  const root = window.document.getElementById("view");
  root.querySelector("[data-new-cap]")?.addEventListener("click", async () => {
    const modalRoot = document.getElementById("modal-root");
    if (!modalRoot) return;
    openNewCapabilityModal(modalRoot, {
      projects: [{ id: project.id, name: project.name }],
      preselectProjectId: project.id,
      onCreated: () => go(currentRoute().path),
    });
  });
  root.querySelector("[data-edit-proj]")?.addEventListener("click", () => {
    const slot = root.querySelector("[data-edit-form]");
    slot.innerHTML = renderEditForm(project);
    slot.classList.remove("hidden");
    slot.querySelector("[data-cancel-edit]")?.addEventListener("click", () => {
      slot.classList.add("hidden");
    });
    slot.querySelector("[data-edit-proj-form]")?.addEventListener("submit", async (event) => {
      event.preventDefault();
      const data = new FormData(event.target);
      const name = String(data.get("name") || "").trim();
      const description = String(data.get("description") || "").trim();
      const errSlot = slot.querySelector("[data-edit-error]");
      errSlot.classList.add("hidden");
      errSlot.innerHTML = "";
      if (!name) {
        errSlot.innerHTML = inlineBanner({ tone: "danger", message: "Name is required." });
        errSlot.classList.remove("hidden");
        return;
      }
      const result = await api.updateProject(project.id, { name, description: description || null });
      if (!result.ok) {
        errSlot.innerHTML = inlineBanner({ tone: "danger", message: `Update failed: ${apiStatusLabel(result)}` });
        errSlot.classList.remove("hidden");
        return;
      }
      go(currentRoute().path);
    });
  });
  root.querySelector("[data-delete-proj]")?.addEventListener("click", async () => {
    if (!window.confirm(`Delete project "${project.name}"? Capabilities under this project will also be removed.`)) return;
    const result = await api.deleteProject(project.id);
    if (!result.ok) {
      window.alert(`Delete failed: ${apiStatusLabel(result)}`);
      return;
    }
    if (workspace) {
      window.location.hash = `#/workspaces/${workspace.id}`;
    } else {
      window.location.hash = "#/capabilities";
    }
    window.location.reload();
  });

  root.querySelectorAll("[data-cap-delete]").forEach((b) => b.addEventListener("click", async () => {
    const id = b.dataset.capDelete;
    const name = b.dataset.capName || id;
    if (!window.confirm(`Delete capability "${name}"? This cannot be undone.`)) return;
    const result = await api.deleteCapability(id);
    if (!result.ok) { window.alert(`Delete failed: ${apiStatusLabel(result)}`); return; }
    window.location.reload();
  }));
}
