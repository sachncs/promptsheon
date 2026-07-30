// Modal for creating a new workspace. Surfaces the
// POST /api/v1/workspaces endpoint that the dashboard was
// previously telling users to call via curl.
import * as api from "../api.js";
import { escape, apiStatusLabel } from "../utils.js";

export async function openWorkspaceCreateModal(root, { onCreated } = {}) {
  function render(opts = {}) {
    return `<div class="modal-backdrop" role="presentation">
      <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="wcw-title">
        <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
          <div>
            <div class="eyebrow">Workspace</div>
            <h2 id="wcw-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">New workspace</h2>
            <p class="mt-1 text-[.7rem] text-muted">A workspace is the top-level container for projects and capabilities.</p>
          </div>
          <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
        </div>
        <form id="workspace-create-form" class="space-y-3 px-5 py-5 sm:px-6">
          ${opts.error ? `<p class="rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800">${escape(opts.error)}</p>` : ""}
          <div>
            <label class="eyebrow mb-2 block" for="ws-name">Name</label>
            <input id="ws-name" name="name" class="field" required autofocus placeholder="e.g. Acme Production" />
          </div>
          <div>
            <label class="eyebrow mb-2 block" for="ws-org">Organization (optional)</label>
            <input id="ws-org" name="organization" class="field" placeholder="e.g. acme.com" />
          </div>
          <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
            <button type="button" class="quiet-button" data-close-modal>Cancel</button>
            <button type="submit" class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>Create workspace</button>
          </div>
        </form>
      </section>
    </div>`;
  }

  function attach() {
    root.querySelector("[data-close-modal]")?.addEventListener("click", () => root.replaceChildren());
    root.querySelector("#workspace-create-form")?.addEventListener("submit", async (event) => {
      event.preventDefault();
      const data = new FormData(event.target);
      const name = String(data.get("name") || "").trim();
      const organization = String(data.get("organization") || "").trim();
      if (!name) {
        root.innerHTML = render({ error: "Name is required." });
        attach();
        return;
      }
      const result = await api.createWorkspace(name, organization || undefined);
      if (!result.ok) {
        root.innerHTML = render({ error: `Could not create workspace: ${escape(apiStatusLabel(result))}` });
        attach();
        return;
      }
      root.replaceChildren();
      if (typeof onCreated === "function") onCreated(result.data);
    });
  }

  root.innerHTML = render();
  attach();
}