// Project detail page. Shows the project's metadata and the
// capabilities it contains. Edit + delete actions open
// inline forms; the delete button asks for confirmation
// before firing DELETE /api/v1/projects/{id}.
import * as api from "../api.js";
import { escape, formatRelative, apiStatusLabel } from "../utils.js";
import { go, currentRoute } from "../router.js";
import { openNewCapabilityModal } from "./new-capability-modal.js";

const STATUS_TONES = { active: "good", approved: "good", pending: "warn", superseded: "neutral", rolled_back: "danger" };

function pill(text, tone = "neutral") {
  return `<span class="status-pill ${tone} !px-2 !py-1"><span class="status-dot"></span>${escape(text)}</span>`;
}

export async function renderProjectDetail(route) {
  const root = window.document.getElementById("view");
  const id = route?.params?.id;
  if (!root) return "";
  if (!id) {
    root.innerHTML = `<p class="panel p-5 mt-5 text-[.78rem] text-muted">No project id in the URL.</p>`;
    return;
  }
  root.innerHTML = `<section class="panel p-5"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
  const projRes = await api.getProject(id);
  if (!projRes.ok) {
    root.innerHTML = `<p class="panel p-5 mt-5 text-[.78rem] text-muted">Could not load project: ${escape(apiStatusLabel(projRes))}</p>`;
    return;
  }
  const project = projRes.data;
  // Capabilities are workspace-scoped at the backend but stored
  // per-project. Load them via the existing listCapabilities
  // endpoint.
  const capsRes = await api.listCapabilities(id);
  const capabilities = capsRes.ok ? (capsRes.data || []) : [];
  // Pull workspace metadata for the breadcrumb. Best-effort;
  // failure just renders a minimal header.
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
    ? `<a href="#/workspaces/${escape(workspace.id)}" class="hover:text-ink">${escape(workspace.name)}</a>
       <span class="text-[#b2b3af]">/</span>`
    : "";
  return `<section class="mt-5 space-y-5">
    <article class="panel p-5 sm:p-6">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <div class="eyebrow flex items-center gap-2">
            ${wsCrumb}
            <span>Project</span>
          </div>
          <h1 class="mt-2 text-[1.5rem] font-bold tracking-[-.04em]">${escape(project.name)}</h1>
          <p class="mt-1 text-[.7rem] text-muted mono">${escape(project.id)}</p>
        </div>
        <div class="flex items-center gap-2">
          <button data-edit-proj class="quiet-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-edit"/></svg>Edit</button>
          <button data-delete-proj class="rounded-md bg-rose-50 px-2.5 py-1.5 text-[.66rem] font-bold text-rose-700 hover:bg-rose-100">Delete project</button>
        </div>
      </div>
      <dl class="mt-5 grid gap-3 sm:grid-cols-3">
        <div><dt class="eyebrow">Description</dt><dd class="mt-1 text-[.78rem] font-bold">${escape(project.description || "—")}</dd></div>
        <div><dt class="eyebrow">Workspace</dt><dd class="mt-1 text-[.78rem] font-bold">${workspace ? escape(workspace.name) : (project.workspace_id ? `<a href="#/workspaces/${escape(project.workspace_id)}" class="hover:underline">${escape(project.workspace_id).slice(-8)}</a>` : "—")}</dd></div>
        <div><dt class="eyebrow">Created</dt><dd class="mt-1 text-[.78rem] font-bold">${escape(formatRelative(project.created_at))}</dd></div>
      </dl>
      <p data-edit-form class="hidden mt-4 rounded-xl bg-paper p-4"></p>
    </article>
    <article class="panel p-5 sm:p-6">
      <div class="flex items-center justify-between">
        <div>
          <div class="eyebrow">Capabilities</div>
          <h2 class="mt-1 text-[1rem] font-bold">${capabilities.length} in this project</h2>
        </div>
        <button data-new-cap class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg> New capability</button>
      </div>
      <div data-cap-list class="mt-4">
        ${renderCapList(capabilities)}
      </div>
    </article>
  </section>`;
}

function renderCapList(capabilities) {
  if (!capabilities.length) {
    return `<p class="mt-3 text-[.7rem] text-muted">No capabilities in this project yet.</p>`;
  }
  return `<table class="mt-2 w-full text-[.7rem]"><thead><tr class="text-left text-[.6rem] uppercase tracking-wider text-muted"><th class="py-1 font-bold">Name</th><th class="py-1 font-bold">Description</th><th class="py-1 font-bold">Created</th><th class="py-1 font-bold text-right"></th></tr></thead><tbody>${capabilities.map((c) => `<tr class="border-t border-line/60">
    <td class="py-2"><a href="#/capabilities/${escape(c.id)}" class="font-bold hover:underline">${escape(c.name)}</a><span class="block text-[.62rem] text-muted mono">${escape(c.id)}</span></td>
    <td class="py-2 text-[.66rem] text-muted">${escape(c.description || "—")}</td>
    <td class="py-2 mono">${escape(formatRelative(c.created_at))}</td>
    <td class="py-2 text-right"><button data-cap-delete="${escape(c.id)}" data-cap-name="${escape(c.name)}" class="rounded-md bg-rose-50 px-2 py-1 text-[.62rem] font-bold text-rose-700 hover:bg-rose-100">Delete</button></td>
  </tr>`).join("")}</tbody></table>`;
}

function renderEditForm(project) {
  return `<form data-edit-proj-form class="space-y-3">
    <p class="text-[.7rem] text-muted">Update the project name and description. The id and workspace are immutable.</p>
    <div><label class="eyebrow mb-2 block" for="proj-edit-name">Name</label><input id="proj-edit-name" name="name" class="field" required value="${escape(project.name)}" /></div>
    <div><label class="eyebrow mb-2 block" for="proj-edit-desc">Description</label><textarea id="proj-edit-desc" name="description" class="field" rows="3">${escape(project.description || "")}</textarea></div>
    <p data-edit-error class="hidden rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800"></p>
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
    // Fetch the users list once for the modal's owner dropdown.
    const { openNewCapabilityModal: openModal } = await import("./new-capability-modal.js");
    // The existing new-capability modal expects a `projects` list.
    // Re-use the listCapabilities round-trip via the current
    // project so the modal can pre-select it.
    openModal(modalRoot, {
      projects: capabilities.length > 0 ? [{ id: project.id, name: project.name }] : [{ id: project.id, name: project.name }],
      onCreated: () => go(currentRoute().path)
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
      if (!name) {
        errSlot.textContent = "Name is required.";
        errSlot.classList.remove("hidden");
        return;
      }
      const result = await api.updateProject(project.id, { name, description: description || null });
      if (!result.ok) {
        errSlot.textContent = `Update failed: ${apiStatusLabel(result)}`;
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
    if (workspace) go(`/workspaces/${encodeURIComponent(workspace.id)}`);
    else go("/capabilities");
  });
  root.querySelectorAll("[data-cap-delete]").forEach((b) =>
    b.addEventListener("click", async () => {
      const id = b.dataset.capDelete;
      const name = b.dataset.capName || id;
      if (!window.confirm(`Delete capability "${name}"?`)) return;
      const result = await api.deleteCapability(id);
      if (!result.ok) {
        window.alert(`Delete failed: ${apiStatusLabel(result)}`);
        return;
      }
      go(currentRoute().path);
    })
  );
}