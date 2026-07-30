// Workspace detail page. Shows the workspace's metadata
// (name, organization, timestamps) and a list of its
// projects. Edit + delete actions open inline forms; the
// delete button asks for confirmation before firing the
// DELETE /api/v1/workspaces/{id} request.
import * as api from "../api.js";
import { escape, formatRelative, apiStatusLabel } from "../utils.js";
import { go, currentRoute } from "../router.js";
import { openProjectCreateModal } from "./project-create-modal.js";

const STAT_TONES = { active: "good", draft: "neutral", archived: "neutral" };

function pill(text, tone = "neutral") {
  return `<span class="status-pill ${tone} !px-2 !py-1"><span class="status-dot"></span>${escape(text)}</span>`;
}

export async function renderWorkspaceDetail(route) {
  const root = window.document.getElementById("view");
  const id = route?.params?.id;
  if (!root) return "";
  if (!id) {
    root.innerHTML = `<p class="panel p-5 mt-5 text-[.78rem] text-muted">No workspace id in the URL.</p>`;
    return;
  }
  // Skeleton while we fetch.
  root.innerHTML = `<section class="panel p-5"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
  const [wsRes, projectsRes] = await Promise.all([
    api.getWorkspace(id),
    api.listProjects(id)
  ]);
  if (!wsRes.ok) {
    root.innerHTML = `<p class="panel p-5 mt-5 text-[.78rem] text-muted">Could not load workspace: ${escape(apiStatusLabel(wsRes))}</p>`;
    return;
  }
  const ws = wsRes.data;
  const projects = projectsRes.ok ? (projectsRes.data || []) : [];
  root.innerHTML = render(ws, projects);
  attach(ws, projects);
}

function render(ws, projects) {
  return `<section class="mt-5 space-y-5">
    <article class="panel p-5 sm:p-6">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <div class="eyebrow flex items-center gap-2">
            <a href="#/capabilities" class="text-muted hover:text-ink">Capabilities</a>
            <span class="text-[#b2b3af]">/</span>
            <span>Workspace</span>
          </div>
          <h1 class="mt-2 text-[1.5rem] font-bold tracking-[-.04em]" data-ws-name>${escape(ws.name)}</h1>
          <p class="mt-1 text-[.7rem] text-muted mono" data-ws-id>${escape(ws.id)}</p>
        </div>
        <div class="flex items-center gap-2">
          <button data-edit-ws class="quiet-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-edit"/></svg>Edit</button>
          <button data-delete-ws class="rounded-md bg-rose-50 px-2.5 py-1.5 text-[.66rem] font-bold text-rose-700 hover:bg-rose-100">Delete workspace</button>
        </div>
      </div>
      <dl class="mt-5 grid gap-3 sm:grid-cols-3">
        <div><dt class="eyebrow">Organization</dt><dd class="mt-1 text-[.78rem] font-bold">${escape(ws.organization || "—")}</dd></div>
        <div><dt class="eyebrow">Created</dt><dd class="mt-1 text-[.78rem] font-bold">${escape(formatRelative(ws.created_at))}</dd></div>
        <div><dt class="eyebrow">Updated</dt><dd class="mt-1 text-[.78rem] font-bold">${escape(formatRelative(ws.updated_at))}</dd></div>
      </dl>
      <p data-edit-form class="hidden mt-4 rounded-xl bg-paper p-4"></p>
    </article>
    <article class="panel p-5 sm:p-6">
      <div class="flex items-center justify-between">
        <div>
          <div class="eyebrow">Projects</div>
          <h2 class="mt-1 text-[1rem] font-bold">${projects.length} in this workspace</h2>
        </div>
        <button data-new-project class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg> New project</button>
      </div>
      <div data-project-list class="mt-4">
        ${renderProjectList(projects, ws.id)}
      </div>
    </article>
  </section>`;
}

function renderProjectList(projects, workspaceId) {
  if (!projects.length) {
    return `<p class="mt-3 text-[.7rem] text-muted">No projects in this workspace yet. Create one to start adding capabilities.</p>`;
  }
  return `<table class="mt-2 w-full text-[.7rem]"><thead><tr class="text-left text-[.6rem] uppercase tracking-wider text-muted"><th class="py-1 font-bold">Name</th><th class="py-1 font-bold">Description</th><th class="py-1 font-bold">Created</th><th class="py-1 font-bold text-right"></th></tr></thead><tbody>${projects.map((p) => `<tr class="border-t border-line/60" data-project-row="${escape(p.id)}">
    <td class="py-2"><a href="#/projects/${escape(p.id)}" class="font-bold hover:underline">${escape(p.name)}</a><span class="block text-[.62rem] text-muted mono">${escape(p.id)}</span></td>
    <td class="py-2 text-[.66rem] text-muted">${escape(p.description || "—")}</td>
    <td class="py-2 mono">${escape(formatRelative(p.created_at))}</td>
    <td class="py-2 text-right"><button data-project-delete="${escape(p.id)}" data-project-name="${escape(p.name)}" class="rounded-md bg-rose-50 px-2 py-1 text-[.62rem] font-bold text-rose-700 hover:bg-rose-100">Delete</button></td>
  </tr>`).join("")}</tbody></table>`;
}

function renderEditForm(ws) {
  return `<form data-edit-ws-form class="space-y-3">
    <p class="text-[.7rem] text-muted">Update the workspace name and organization. The id is immutable.</p>
    <div><label class="eyebrow mb-2 block" for="ws-edit-name">Name</label><input id="ws-edit-name" name="name" class="field" required value="${escape(ws.name)}" /></div>
    <div><label class="eyebrow mb-2 block" for="ws-edit-org">Organization</label><input id="ws-edit-org" name="organization" class="field" value="${escape(ws.organization || "")}" /></div>
    <p data-edit-error class="hidden rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800"></p>
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
      onCreated: () => go(currentRoute().path)
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
      if (!name) {
        errSlot.textContent = "Name is required.";
        errSlot.classList.remove("hidden");
        return;
      }
      const result = await api.updateWorkspace(ws.id, { name, organization: organization || null });
      if (!result.ok) {
        errSlot.textContent = `Update failed: ${apiStatusLabel(result)}`;
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
    go("/capabilities");
  });
  root.querySelectorAll("[data-project-delete]").forEach((b) =>
    b.addEventListener("click", async () => {
      const id = b.dataset.projectDelete;
      const name = b.dataset.projectName || id;
      if (!window.confirm(`Delete project "${name}"? Capabilities under this project will also be removed.`)) return;
      const result = await api.deleteProject(id);
      if (!result.ok) {
        window.alert(`Delete failed: ${apiStatusLabel(result)}`);
        return;
      }
      go(currentRoute().path);
    })
  );
}