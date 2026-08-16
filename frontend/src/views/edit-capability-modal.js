// Edit capability modal. Triggered from the capability detail
// page's Edit button. Sends PUT /api/v1/capabilities/{id} with
// name / description / owner / tags.

import { escape, apiStatusLabel } from "../utils.js";
import * as api from "../api.js";
import { openModal } from "../dialog.js";
import { inlineBanner } from "../ui.js";
import { toast } from "../toast.js";

function formHtml(capability, { error = "" } = {}) {
  const tagsValue = Array.isArray(capability.tags) ? capability.tags.join(", ") : "";
  return `${error ? inlineBanner({ tone: "danger", message: error }) : ""}
    <form id="edit-cap-form" class="space-y-4">
      <div><label class="field-label" for="ec-name">Name</label><input id="ec-name" name="name" class="field" required autofocus value="${escape(capability.name)}" /></div>
      <div><label class="field-label" for="ec-desc">Description</label><textarea id="ec-desc" name="description" class="field min-h-20 resize-y">${escape(capability.description || "")}</textarea></div>
      <div class="grid gap-3 sm:grid-cols-2">
        <div><label class="field-label" for="ec-owner">Owner ID</label><input id="ec-owner" name="owner" class="field mono" value="${escape(capability.owner || "")}" /></div>
        <div><label class="field-label" for="ec-tags">Tags</label><input id="ec-tags" name="tags" class="field" value="${escape(tagsValue)}" placeholder="comma separated" /></div>
      </div>
    </form>`;
}

function footerHtml() {
  return `<button type="button" class="quiet-button" data-close-modal>Cancel</button>
    <button type="submit" form="edit-cap-form" class="primary-button">Save</button>`;
}

export async function openEditCapabilityModal(root, capability) {
  if (!root) return;
  const modal = openModal({
    title: capability.name,
    subtitle: "Capability / Edit",
    body: formHtml(capability),
    footer: footerHtml(),
    size: "narrow",
  });

  function attach() {
    modal.root.querySelector("#edit-cap-form")?.addEventListener("submit", async (event) => {
      event.preventDefault();
      const data = new FormData(event.target);
      const tags = (data.get("tags") || "").toString().split(",").map((t) => t.trim()).filter(Boolean);
      const payload = {
        name: (data.get("name") || "").toString().trim() || undefined,
        description: (data.get("description") || "").toString().trim() || undefined,
        owner: (data.get("owner") || "").toString().trim() || undefined,
        tags: tags.length ? tags : [],
      };
      const result = await api.updateCapability(capability.id, payload);
      if (!result.ok) {
        modal.body.innerHTML = formHtml(capability, { error: apiStatusLabel(result) });
        attach();
        return;
      }
      modal.close();
      toast.success("Capability saved", capability.name);
      setTimeout(() => window.location.reload(), 250);
    });
  }
  attach();
}
