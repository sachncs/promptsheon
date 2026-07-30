// Modal for creating a new project inside a workspace. Surfaces
// the POST /api/v1/workspaces/{id}/projects endpoint that the
// dashboard was previously telling users to call via curl.
import * as api from "../api.js";
import { escape, apiStatusLabel } from "../utils.js";

export async function openProjectCreateModal(root, { workspaces = [], preselectWorkspaceId, onCreated } = {}) {
  function render(opts = {}) {
    const wsOptions = workspaces.map((w) =>
      `<option value="${escape(w.id)}" ${(opts.selectedWorkspaceId || preselectWorkspaceId) === w.id ? "selected" : ""}>${escape(w.name)}</option>`
    ).join("");
    return `<div class="modal-backdrop" role="presentation">
      <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="pcw-title">
        <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
          <div>
            <div class="eyebrow">Project</div>
            <h2 id="pcw-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">New project</h2>
            <p class="mt-1 text-[.7rem] text-muted">A project groups capabilities under a workspace.</p>
          </div>
          <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
        </div>
        <form id="project-create-form" class="space-y-3 px-5 py-5 sm:px-6">
          ${opts.error ? `<p class="rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800">${escape(opts.error)}</p>` : ""}
          ${workspaces.length === 0 ? `<p class="rounded-lg bg-amber-50 px-3 py-2 text-[.68rem] text-amber-800">No workspaces yet. Create a workspace first, then return here.</p>` : ""}
          <div>
            <label class="eyebrow mb-2 block" for="proj-ws">Workspace</label>
            <select id="proj-ws" name="workspace_id" class="field" required ${workspaces.length === 0 ? "disabled" : ""}>
              ${workspaces.length > 0 ? wsOptions : ""}
            </select>
          </div>
          <div>
            <label class="eyebrow mb-2 block" for="proj-name">Name</label>
            <input id="proj-name" name="name" class="field" required autofocus placeholder="e.g. customer-support" />
          </div>
          <div>
            <label class="eyebrow mb-2 block" for="proj-desc">Description (optional)</label>
            <textarea id="proj-desc" name="description" class="field" rows="3" placeholder="What lives in this project?"></textarea>
          </div>
          <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
            <button type="button" class="quiet-button" data-close-modal>Cancel</button>
            <button type="submit" class="primary-button" ${workspaces.length === 0 ? "disabled" : ""}><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>Create project</button>
          </div>
        </form>
      </section>
    </div>`;
  }

  function attach() {
    root.querySelector("[data-close-modal]")?.addEventListener("click", () => root.replaceChildren());
    root.querySelector("#project-create-form")?.addEventListener("submit", async (event) => {
      event.preventDefault();
      const data = new FormData(event.target);
      const workspaceId = String(data.get("workspace_id") || "").trim();
      const name = String(data.get("name") || "").trim();
      const description = String(data.get("description") || "").trim();
      if (!workspaceId) {
        root.innerHTML = render({ selectedWorkspaceId: "", error: "Pick a workspace." });
        attach();
        return;
      }
      if (!name) {
        root.innerHTML = render({ selectedWorkspaceId: workspaceId, error: "Name is required." });
        attach();
        return;
      }
      const result = await api.createProject(workspaceId, name, description || undefined);
      if (!result.ok) {
        root.innerHTML = render({ selectedWorkspaceId: workspaceId, error: `Could not create project: ${escape(apiStatusLabel(result))}` });
        attach();
        return;
      }
      root.replaceChildren();
      if (typeof onCreated === "function") onCreated(result.data, workspaceId);
    });
  }

  root.innerHTML = render();
  attach();
}