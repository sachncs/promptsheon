// User-edit modal. Triggered from the operations → users tab
// "Edit" buttons. Sends PUT /api/v1/users/{id} with the
// updated email + name + role.

import * as api from "../api.js";
import { apiStatusLabel } from "../utils.js";
import { openModal } from "../dialog.js";
import { inlineBanner } from "../ui.js";
import { toast } from "../toast.js";

function formHtml(user, { error = "" } = {}) {
  const initial = user || {};
  const v = {
    email: initial.email || "",
    name: initial.name || "",
    role: initial.role || "reader",
  };
  return `${error ? inlineBanner({ tone: "danger", message: error }) : ""}
    <form id="user-edit-form" class="space-y-3">
      <div><label class="field-label" for="ue-email">Email</label><input id="ue-email" name="email" type="email" class="field" required value="${v.email}" autofocus /></div>
      <div><label class="field-label" for="ue-name">Name</label><input id="ue-name" name="name" class="field" required value="${v.name}" /></div>
      <div><label class="field-label" for="ue-role">Role</label>
        <select id="ue-role" name="role" class="field" required>
          <option value="admin" ${v.role === "admin" ? "selected" : ""}>admin</option>
          <option value="writer" ${v.role === "writer" ? "selected" : ""}>writer</option>
          <option value="reader" ${v.role === "reader" ? "selected" : ""}>reader</option>
        </select>
      </div>
    </form>`;
}

function footerHtml() {
  return `<button type="button" class="quiet-button" data-close-modal>Cancel</button>
    <button type="submit" form="user-edit-form" class="primary-button">Save</button>`;
}

export async function openUserEditModal(root, user) {
  if (!user) return;
  const id = user.id || user.user_id;
  const modal = openModal({
    title: `Edit ${user.email || id}`,
    subtitle: id,
    body: formHtml(user),
    footer: footerHtml(),
    size: "narrow",
  });

  function attach() {
    modal.root.querySelector("#user-edit-form")?.addEventListener("submit", async (event) => {
      event.preventDefault();
      const data = new FormData(event.target);
      const email = String(data.get("email") || "").trim();
      const name = String(data.get("name") || "").trim();
      const role = String(data.get("role") || "");
      if (!email || !name || !role) {
        modal.body.innerHTML = formHtml({ ...user, email, name, role }, { error: "Email, name, and role are all required." });
        attach();
        return;
      }
      const result = await api.updateUser(id, { email, name, role });
      if (!result.ok) {
        modal.body.innerHTML = formHtml({ ...user, email, name, role }, { error: `Could not save: ${apiStatusLabel(result)}` });
        attach();
        return;
      }
      modal.close();
      toast.success("User updated", email);
      setTimeout(() => window.location.reload(), 250);
    });
  }
  attach();
}
