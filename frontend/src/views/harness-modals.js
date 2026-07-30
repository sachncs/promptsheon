// Harness modals: dataset create, precondition create, precondition
// edit, dataset cases (bulk-write). All take the modal-root
// element + a capability or id and mutate the DOM directly.
import * as api from "../api.js";
import { escape, apiStatusLabel } from "../utils.js";

function field(label, name, value, placeholder, type = "text") {
  return `<div><label class="eyebrow mb-2 block" for="hf-${escape(name)}">${escape(label)}</label><input id="hf-${escape(name)}" name="${escape(name)}" type="${type}" class="field" required value="${escape(value || "")}" placeholder="${escape(placeholder || "")}" /></div>`;
}

function textarea(label, name, value, placeholder, rows = 4) {
  return `<div><label class="eyebrow mb-2 block" for="hf-${escape(name)}">${escape(label)}</label><textarea id="hf-${escape(name)}" name="${escape(name)}" class="field mono min-h-24 resize-y" rows="${rows}" required placeholder="${escape(placeholder || "")}">${escape(value || "")}</textarea></div>`;
}

function mount(root, body) {
  root.innerHTML = body;
  root.querySelectorAll("[data-close-modal]").forEach((b) =>
    b.addEventListener("click", () => root.replaceChildren())
  );
}

function errorSlot(name = "error") {
  return `<p id="${name}" class="hidden rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800"></p>`;
}

function showError(slot, message) {
  slot.textContent = message;
  slot.classList.remove("hidden");
}

export async function openDatasetCreateModal(root, capability) {
  function render(opts = {}) {
    return `<div class="modal-backdrop" role="presentation">
      <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="ds-new-title">
        <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
          <div>
            <div class="eyebrow">Dataset</div>
            <h2 id="ds-new-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">New dataset</h2>
            <p class="mt-1 text-[.7rem] text-muted">Eval cases for <span class="font-bold text-ink">${escape(capability.name)}</span>.</p>
          </div>
          <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
        </div>
        <form id="dataset-create-form" class="space-y-3 px-5 py-5 sm:px-6">
          ${opts.error || errorSlot()}
          ${field("Name", "name", "", "e.g. greeting-scenarios")}
          ${textarea("Description (optional)", "description", "", "What does this dataset cover?", 3)}
          <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
            <button type="button" class="quiet-button" data-close-modal>Cancel</button>
            <button type="submit" class="primary-button">Create dataset</button>
          </div>
        </form>
      </section>
    </div>`;
  }
  mount(root, render());
  root.querySelector("#dataset-create-form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const data = new FormData(event.target);
    const name = String(data.get("name") || "").trim();
    const description = String(data.get("description") || "").trim();
    const errSlot = root.querySelector("#error");
    errSlot.classList.add("hidden");
    if (!name) {
      showError(errSlot, "Name is required.");
      return;
    }
    const result = await api.createDataset(capability.id, { name, description: description || undefined });
    if (!result.ok) {
      root.innerHTML = render({ error: `Could not create dataset: ${escape(apiStatusLabel(result))}` });
      openDatasetCreateModalAttach(root, capability);
      return;
    }
    window.location.reload();
  });
}

function openDatasetCreateModalAttach(root, capability) {
  root.querySelector("#dataset-create-form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const data = new FormData(event.target);
    const name = String(data.get("name") || "").trim();
    const description = String(data.get("description") || "").trim();
    if (!name) return;
    const result = await api.createDataset(capability.id, { name, description: description || undefined });
    if (!result.ok) {
      const err = root.querySelector("#error");
      err.textContent = `Could not create dataset: ${apiStatusLabel(result)}`;
      err.classList.remove("hidden");
      return;
    }
    window.location.reload();
  });
  root.querySelectorAll("[data-close-modal]").forEach((b) =>
    b.addEventListener("click", () => root.replaceChildren())
  );
}

export async function openDatasetCasesModal(root, datasetId) {
  // Load the current cases (via the dataset detail) so the user
  // sees what they're about to overwrite. PUT replaces the
  // case list entirely.
  mount(root, `<div class="modal-backdrop" role="presentation">
    <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="ds-cases-title">
      <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
        <div>
          <div class="eyebrow">Dataset cases</div>
          <h2 id="ds-cases-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em] mono">${escape(datasetId)}</h2>
        </div>
        <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
      </div>
      <form id="dataset-cases-form" class="space-y-3 px-5 py-5 sm:px-6">
        ${errorSlot()}
        <p class="text-[.7rem] text-muted">PUT replaces the case list. Each case is a JSON object with <code>inputs</code> (object) and <code>expected</code> (any) keys.</p>
        ${textarea("Cases (JSON array)", "cases", "[]", '[{"inputs": {"prompt": "hello"}, "expected": "hi"}]', 12)}
        <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
          <button type="button" class="quiet-button" data-close-modal>Cancel</button>
          <button type="submit" class="primary-button">Save cases</button>
        </div>
      </form>
    </section>
  </div>`);

  // Try to load current cases for edit-in-place convenience.
  const detail = await api.getDataset(datasetId);
  if (detail.ok && Array.isArray(detail.data?.cases) && detail.data.cases.length) {
    const ta = root.querySelector("textarea[name=cases]");
    if (ta) ta.value = JSON.stringify(detail.data.cases, null, 2);
  }

  root.querySelector("#dataset-cases-form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const data = new FormData(event.target);
    const raw = String(data.get("cases") || "").trim();
    const errSlot = root.querySelector("#error");
    errSlot.classList.add("hidden");
    let cases;
    try { cases = JSON.parse(raw); } catch (e) {
      showError(errSlot, "Cases must be valid JSON.");
      return;
    }
    if (!Array.isArray(cases)) {
      showError(errSlot, "Cases must be a JSON array.");
      return;
    }
    const result = await api.putDatasetCases(datasetId, cases);
    if (!result.ok) {
      showError(errSlot, `Save failed: ${apiStatusLabel(result)}`);
      return;
    }
    window.location.reload();
  });
}

export async function openPreconditionCreateModal(root, capability) {
  function render(opts = {}) {
    return `<div class="modal-backdrop" role="presentation">
      <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="pc-new-title">
        <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
          <div>
            <div class="eyebrow">Precondition</div>
            <h2 id="pc-new-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">New precondition</h2>
            <p class="mt-1 text-[.7rem] text-muted">A shell command that must pass before a release activates for <span class="font-bold text-ink">${escape(capability.name)}</span>.</p>
          </div>
          <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
        </div>
        <form id="precondition-create-form" class="space-y-3 px-5 py-5 sm:px-6">
          ${opts.error || errorSlot()}
          ${field("Name", "name", "", "e.g. integration-tests")}
          ${textarea("Shell command", "command", "", 'curl -fsS http://localhost:8080/health | grep -q healthy', 4)}
          ${field("Timeout (s)", "timeout_seconds", "60", "60", "number")}
          <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
            <button type="button" class="quiet-button" data-close-modal>Cancel</button>
            <button type="submit" class="primary-button">Create precondition</button>
          </div>
        </form>
      </section>
    </div>`;
  }
  mount(root, render());
  root.querySelector("#precondition-create-form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const data = new FormData(event.target);
    const name = String(data.get("name") || "").trim();
    const command = String(data.get("command") || "").trim();
    const timeout = Number(data.get("timeout_seconds") || 60);
    const errSlot = root.querySelector("#error");
    errSlot.classList.add("hidden");
    if (!name || !command) {
      showError(errSlot, "Name and command are required.");
      return;
    }
    const result = await api.createPrecondition(capability.id, { name, command, timeout_seconds: timeout });
    if (!result.ok) {
      root.innerHTML = render({ error: `Could not create precondition: ${escape(apiStatusLabel(result))}` });
      attachPreconditionCreate(root, capability);
      return;
    }
    window.location.reload();
  });
}

function attachPreconditionCreate(root, capability) {
  root.querySelector("#precondition-create-form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const data = new FormData(event.target);
    const name = String(data.get("name") || "").trim();
    const command = String(data.get("command") || "").trim();
    const timeout = Number(data.get("timeout_seconds") || 60);
    if (!name || !command) return;
    const result = await api.createPrecondition(capability.id, { name, command, timeout_seconds: timeout });
    if (!result.ok) {
      const err = root.querySelector("#error");
      err.textContent = `Could not create precondition: ${apiStatusLabel(result)}`;
      err.classList.remove("hidden");
      return;
    }
    window.location.reload();
  });
  root.querySelectorAll("[data-close-modal]").forEach((b) =>
    b.addEventListener("click", () => root.replaceChildren())
  );
}

export async function openPreconditionEditModal(root, id, existing) {
  const pre = existing || { id, name: "", command: "", timeout_seconds: 60 };
  function render(opts = {}) {
    return `<div class="modal-backdrop" role="presentation">
      <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="pc-edit-title">
        <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
          <div>
            <div class="eyebrow">Precondition</div>
            <h2 id="pc-edit-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">Edit ${escape(pre.name || pre.id)}</h2>
          </div>
          <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
        </div>
        <form id="precondition-edit-form" class="space-y-3 px-5 py-5 sm:px-6">
          ${opts.error || errorSlot()}
          ${field("Name", "name", pre.name, "")}
          ${textarea("Shell command", "command", pre.command, "", 4)}
          ${field("Timeout (s)", "timeout_seconds", String(pre.timeout_seconds || 60), "", "number")}
          <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
            <button type="button" class="quiet-button" data-close-modal>Cancel</button>
            <button type="submit" class="primary-button">Save</button>
          </div>
        </form>
      </section>
    </div>`;
  }
  mount(root, render());
  root.querySelector("#precondition-edit-form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const data = new FormData(event.target);
    const name = String(data.get("name") || "").trim();
    const command = String(data.get("command") || "").trim();
    const timeout = Number(data.get("timeout_seconds") || 60);
    const errSlot = root.querySelector("#error");
    errSlot.classList.add("hidden");
    if (!name || !command) {
      showError(errSlot, "Name and command are required.");
      return;
    }
    const result = await api.updatePrecondition(id, { name, command, timeout_seconds: timeout });
    if (!result.ok) {
      root.innerHTML = render({ error: `Could not save precondition: ${escape(apiStatusLabel(result))}` });
      attachPreconditionEdit(root, id, pre);
      return;
    }
    window.location.reload();
  });
}

function attachPreconditionEdit(root, id, pre) {
  root.querySelector("#precondition-edit-form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const data = new FormData(event.target);
    const name = String(data.get("name") || "").trim();
    const command = String(data.get("command") || "").trim();
    const timeout = Number(data.get("timeout_seconds") || 60);
    if (!name || !command) return;
    const result = await api.updatePrecondition(id, { name, command, timeout_seconds: timeout });
    if (!result.ok) {
      const err = root.querySelector("#error");
      err.textContent = `Could not save precondition: ${apiStatusLabel(result)}`;
      err.classList.remove("hidden");
      return;
    }
    window.location.reload();
  });
  root.querySelectorAll("[data-close-modal]").forEach((b) =>
    b.addEventListener("click", () => root.replaceChildren())
  );
}