// Modal for creating a new workspace. Surfaces the
// POST /api/v1/workspaces endpoint that the dashboard was
// previously telling users to call via curl.

import * as api from "../api.js";
import { escape, apiStatusLabel } from "../utils.js";
import { openModal } from "../dialog.js";
import { inlineBanner } from "../ui.js";
import { toast } from "../toast.js";

export async function openWorkspaceCreateModal(root, { onCreated } = {}) {
  function formHtml({ error = "" } = {}) {
    return `${error ? inlineBanner({ tone: "danger", message: error }) : ""}
      <form id="workspace-create-form" class="space-y-3">
        <div>
          <label class="field-label" for="ws-name">Name</label>
          <input id="ws-name" name="name" class="field" required autofocus placeholder="e.g. Acme Production" />
        </div>
        <div>
          <label class="field-label" for="ws-org">Organization (optional)</label>
          <input id="ws-org" name="organization" class="field" placeholder="e.g. acme.com" />
        </div>
      </form>`;
  }

  function footerHtml() {
    return `<button type="button" class="quiet-button" data-close-modal>Cancel</button>
      <button type="submit" form="workspace-create-form" class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>Create workspace</button>`;
  }

  const modal = openModal({
    title: "New workspace",
    subtitle: "A workspace is the top-level container for projects and capabilities.",
    body: formHtml(),
    footer: footerHtml(),
    size: "narrow",
  });

  function attach() {
    modal.root.querySelector("#workspace-create-form")?.addEventListener("submit", async (event) => {
      event.preventDefault();
      const data = new FormData(event.target);
      const name = String(data.get("name") || "").trim();
      const organization = String(data.get("organization") || "").trim();
      if (!name) {
        modal.body.innerHTML = formHtml({ error: "Name is required." });
        attach();
        return;
      }
      const result = await api.createWorkspace(name, organization || undefined);
      if (!result.ok) {
        modal.body.innerHTML = formHtml({ error: `Could not create workspace: ${apiStatusLabel(result)}` });
        attach();
        return;
      }
      modal.close();
      toast.success("Workspace created", result.data?.name || "");
      if (typeof onCreated === "function") onCreated(result.data);
    });
  }
  attach();
}
