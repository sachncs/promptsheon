// User-edit modal. Triggered from the operations → users tab
// "Edit" buttons. Sends PUT /api/v1/users/{id} with the
// updated email + name + role.
import * as api from "../api.js";
import { escape, apiStatusLabel } from "../utils.js";

function showError(slot, message) {
  slot.textContent = message;
  slot.classList.remove("hidden");
}

export async function openUserEditModal(root, user) {
  if (!user) return;
  const id = user.id || user.user_id;
  const initialEmail = user.email || "";
  const initialName = user.name || "";
  const initialRole = user.role || "reader";

  function render(opts = {}) {
    return `<div class="modal-backdrop" role="presentation">
      <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="ue-title">
        <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
          <div>
            <div class="eyebrow">User</div>
            <h2 id="ue-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">Edit ${escape(initialEmail || id)}</h2>
            <p class="mt-1 text-[.7rem] text-muted mono">${escape(id)}</p>
          </div>
          <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
        </div>
        <form id="user-edit-form" class="space-y-3 px-5 py-5 sm:px-6">
          <p id="user-edit-error" class="hidden rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800"></p>
          <div><label class="eyebrow mb-2 block" for="ue-email">Email</label><input id="ue-email" name="email" type="email" class="field" required value="${escape(opts.email ?? initialEmail)}" /></div>
          <div><label class="eyebrow mb-2 block" for="ue-name">Name</label><input id="ue-name" name="name" class="field" required value="${escape(opts.name ?? initialName)}" /></div>
          <div><label class="eyebrow mb-2 block" for="ue-role">Role</label>
            <select id="ue-role" name="role" class="field" required>
              <option value="admin" ${(opts.role ?? initialRole) === "admin" ? "selected" : ""}>admin</option>
              <option value="writer" ${(opts.role ?? initialRole) === "writer" ? "selected" : ""}>writer</option>
              <option value="reader" ${(opts.role ?? initialRole) === "reader" ? "selected" : ""}>reader</option>
            </select>
          </div>
          <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
            <button type="button" class="quiet-button" data-close-modal>Cancel</button>
            <button type="submit" class="primary-button">Save</button>
          </div>
        </form>
      </section>
    </div>`;
  }
  root.innerHTML = render();
  root.querySelectorAll("[data-close-modal]").forEach((b) => b.addEventListener("click", () => root.replaceChildren()));
  root.querySelector("#user-edit-form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const data = new FormData(event.target);
    const email = String(data.get("email") || "").trim();
    const name = String(data.get("name") || "").trim();
    const role = String(data.get("role") || "");
    const errSlot = root.querySelector("#user-edit-error");
    errSlot.classList.add("hidden");
    if (!email || !name || !role) {
      showError(errSlot, "Email, name, and role are all required.");
      return;
    }
    const result = await api.updateUser(id, { email, name, role });
    if (!result.ok) {
      root.innerHTML = render({ email, name, role, error: `Could not save: ${escape(apiStatusLabel(result))}` });
      openUserEditModal(root, { id, email, name, role });
      return;
    }
    window.location.reload();
  });
}