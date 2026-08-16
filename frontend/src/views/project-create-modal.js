// Modal for creating a new project inside a workspace. Surfaces
// the POST /api/v1/workspaces/{id}/projects endpoint that the
// dashboard was previously telling users to call via curl.

import * as api from "../api.js";
import { escape, apiStatusLabel } from "../utils.js";
import { openModal } from "../dialog.js";
import { inlineBanner } from "../ui.js";
import { toast } from "../toast.js";

export async function openProjectCreateModal(root, { workspaces = [], preselectWorkspaceId, onCreated } = {}) {
  const buildBody = ({ error = "", selectedWorkspaceId = preselectWorkspaceId } = {}) => {
    const wsOptions = workspaces.map((w) => `<option value="${escape(w.id)}" ${(selectedWorkspaceId || preselectWorkspaceId) === w.id ? "selected" : ""}>${escape(w.name)}</option>`).join("");
    return `${error ? inlineBanner({ tone: "danger", message: error }) : ""}
      ${workspaces.length === 0 ? inlineBanner({ tone: "warn", message: "No workspaces yet. Create a workspace first, then return here." }) : ""}
      <form id="project-create-form" class="space-y-3">
        <div>
          <label class="field-label" for="proj-ws">Workspace</label>
          <select id="proj-ws" name="workspace_id" class="field" required ${workspaces.length === 0 ? "disabled" : ""}>${workspaces.length > 0 ? wsOptions : ""}</select>
        </div>
        <div>
          <label class="field-label" for="proj-name">Name</label>
          <input id="proj-name" name="name" class="field" required autofocus placeholder="e.g. customer-support" />
        </div>
        <div>
          <label class="field-label" for="proj-desc">Description (optional)</label>
          <textarea id="proj-desc" name="description" class="field" rows="3" placeholder="What lives in this project?"></textarea>
        </div>
      </form>`;
  };

  const buildFooter = () => `<button type="button" class="quiet-button" data-close-modal>Cancel</button>
    <button type="submit" form="project-create-form" class="primary-button" ${workspaces.length === 0 ? "disabled" : ""}><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>Create project</button>`;

  const modal = openModal({
    title: "New project",
    subtitle: "A project groups capabilities under a workspace.",
    body: buildBody(),
    footer: buildFooter(),
    size: "narrow",
  });

  function attach() {
    modal.root.querySelector("#project-create-form")?.addEventListener("submit", async (event) => {
      event.preventDefault();
      const data = new FormData(event.target);
      const workspaceId = String(data.get("workspace_id") || "").trim();
      const name = String(data.get("name") || "").trim();
      const description = String(data.get("description") || "").trim();
      if (!workspaceId) {
        modal.body.innerHTML = buildBody({ error: "Pick a workspace." });
        attach();
        return;
      }
      if (!name) {
        modal.body.innerHTML = buildBody({ selectedWorkspaceId: workspaceId, error: "Name is required." });
        attach();
        return;
      }
      const result = await api.createProject(workspaceId, name, description || undefined);
      if (!result.ok) {
        modal.body.innerHTML = buildBody({ selectedWorkspaceId: workspaceId, error: `Could not create project: ${apiStatusLabel(result)}` });
        attach();
        return;
      }
      modal.close();
      toast.success("Project created", result.data?.name || "");
      if (typeof onCreated === "function") onCreated(result.data, workspaceId);
    });
  }
  attach();
}
