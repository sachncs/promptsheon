import { escape, apiStatusLabel } from "../utils.js";
import * as api from "../api.js";

export async function openEditCapabilityModal(root, capability) {
  if (!root) return;
  const tagsValue = Array.isArray(capability.tags) ? capability.tags.join(", ") : "";
  const render = ({ error = null } = {}) => `<div class="modal-backdrop" role="presentation">
    <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="edit-cap-title">
      <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
        <div><div class="eyebrow">Capability / Edit</div><h2 id="edit-cap-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">${escape(capability.name)}</h2></div>
        <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
      </div>
      <form id="edit-cap-form" class="space-y-4 px-5 py-5 sm:px-6">
        <div><label class="eyebrow mb-2 block" for="ec-name">Name</label><input id="ec-name" name="name" class="field" required data-autofocus value="${escape(capability.name)}" /></div>
        <div><label class="eyebrow mb-2 block" for="ec-desc">Description</label><textarea id="ec-desc" name="description" class="field min-h-20 resize-y">${escape(capability.description || "")}</textarea></div>
        <div class="grid gap-3 sm:grid-cols-2">
          <div><label class="eyebrow mb-2 block" for="ec-owner">Owner ID</label><input id="ec-owner" name="owner" class="field mono" value="${escape(capability.owner || "")}" /></div>
          <div><label class="eyebrow mb-2 block" for="ec-tags">Tags</label><input id="ec-tags" name="tags" class="field" value="${escape(tagsValue)}" placeholder="comma separated" /></div>
        </div>
        ${error ? `<p class="rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800">${escape(error)}</p>` : ""}
        <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
          <button type="button" class="quiet-button" data-close-modal>Cancel</button>
          <button type="submit" class="primary-button">Save</button>
        </div>
      </form>
    </section>
  </div>`;
  root.innerHTML = render();
  attach(root, render, capability);
}

function attach(root, render, capability) {
  root.querySelector("[data-close-modal]")?.addEventListener("click", () => root.replaceChildren());
  root.querySelector("#edit-cap-form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const data = new FormData(event.target);
    const tags = (data.get("tags") || "").toString().split(",").map((t) => t.trim()).filter(Boolean);
    const payload = {
      name: (data.get("name") || "").toString().trim() || undefined,
      description: (data.get("description") || "").toString().trim() || undefined,
      owner: (data.get("owner") || "").toString().trim() || undefined,
      tags: tags.length ? tags : []
    };
    const result = await api.updateCapability(capability.id, payload);
    if (!result.ok) {
      root.innerHTML = render({ error: apiStatusLabel(result) });
      attach(root, render, capability);
      return;
    }
    window.location.reload();
  });
}
