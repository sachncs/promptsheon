// Harness modals: dataset create, dataset cases, precondition
// create, precondition edit. All four go through the shared
// dialog system so title / body / footer / cancel / close / ESC
// / focus-trap behavior matches every other dashboard modal.

import * as api from "../api.js";
import { apiStatusLabel } from "../utils.js";
import { openModal } from "../dialog.js";
import { inlineBanner } from "../ui.js";
import { toast } from "../toast.js";

function field(label, name, value, placeholder, type = "text") {
  return `<div><label class="field-label" for="hf-${name}">${label}</label><input id="hf-${name}" name="${name}" type="${type}" class="field" required value="${value || ""}" placeholder="${placeholder || ""}" /></div>`;
}

function textarea(label, name, value, placeholder, rows = 4) {
  return `<div><label class="field-label" for="hf-${name}">${label}</label><textarea id="hf-${name}" name="${name}" class="field mono min-h-24 resize-y" rows="${rows}" required placeholder="${placeholder || ""}">${value || ""}</textarea></div>`;
}

function errorBanner(error) {
  return error ? inlineBanner({ tone: "danger", message: error }) : "";
}

function closeAndReload(modal, message) {
  modal.close();
  toast.success(message);
  setTimeout(() => window.location.reload(), 250);
}

function closeAndRetry(modal, errorHtml) {
  // Re-render only the body — focus stays inside the dialog.
  const newBody = modal.body;
  newBody.innerHTML = errorHtml;
}

// ---- Dataset create ----------------------------------------------------

export async function openDatasetCreateModal(root, capability) {
  const formHtml = ({ error = "" } = {}) => `${errorBanner(error)}
    <form id="dataset-create-form" class="space-y-3">
      ${field("Name", "name", "", "e.g. greeting-scenarios")}
      ${textarea("Description (optional)", "description", "", "What does this dataset cover?", 3)}
    </form>`;
  const footer = `<button type="button" class="quiet-button" data-close-modal>Cancel</button>
    <button type="submit" form="dataset-create-form" class="primary-button">Create dataset</button>`;

  const modal = openModal({
    title: "New dataset",
    subtitle: `Eval cases for <span class="font-bold text-ink">${capability.name}</span>.`,
    body: formHtml(),
    footer,
    size: "narrow",
  });

  function attach() {
    modal.root.querySelector("#dataset-create-form")?.addEventListener("submit", async (event) => {
      event.preventDefault();
      const data = new FormData(event.target);
      const name = String(data.get("name") || "").trim();
      const description = String(data.get("description") || "").trim();
      if (!name) {
        modal.body.innerHTML = formHtml({ error: "Name is required." });
        attach();
        return;
      }
      const result = await api.createDataset(capability.id, { name, description: description || undefined });
      if (!result.ok) {
        modal.body.innerHTML = formHtml({ error: `Could not create dataset: ${apiStatusLabel(result)}` });
        attach();
        return;
      }
      closeAndReload(modal, "Dataset created");
    });
  }
  attach();
}

// ---- Dataset cases (bulk write) ---------------------------------------

export async function openDatasetCasesModal(root, datasetId) {
  const formHtml = ({ error = "" } = {}) => `${errorBanner(error)}
    <p class="text-[.7rem] text-muted">PUT replaces the case list. Each case is a JSON object with <code>inputs</code> (object) and <code>expected</code> (any) keys.</p>
    <form id="dataset-cases-form" class="space-y-3">
      ${textarea("Cases (JSON array)", "cases", "[]", '[{"inputs": {"prompt": "hello"}, "expected": "hi"}]', 12)}
    </form>`;
  const footer = `<button type="button" class="quiet-button" data-close-modal>Cancel</button>
    <button type="submit" form="dataset-cases-form" class="primary-button">Save cases</button>`;

  const modal = openModal({
    title: `Dataset cases`,
    subtitle: datasetId,
    body: formHtml(),
    footer,
    size: "wide",
  });

  // Load current cases so the user sees what they're replacing.
  const detail = await api.getDataset(datasetId);
  if (detail.ok && Array.isArray(detail.data?.cases) && detail.data.cases.length) {
    const ta = modal.root.querySelector("textarea[name=cases]");
    if (ta) ta.value = JSON.stringify(detail.data.cases, null, 2);
  }

  function attach() {
    modal.root.querySelector("#dataset-cases-form")?.addEventListener("submit", async (event) => {
      event.preventDefault();
      const data = new FormData(event.target);
      const raw = String(data.get("cases") || "").trim();
      let cases;
      try { cases = JSON.parse(raw); }
      catch (e) {
        modal.body.innerHTML = formHtml({ error: "Cases must be valid JSON." });
        attach();
        return;
      }
      if (!Array.isArray(cases)) {
        modal.body.innerHTML = formHtml({ error: "Cases must be a JSON array." });
        attach();
        return;
      }
      const result = await api.putDatasetCases(datasetId, cases);
      if (!result.ok) {
        modal.body.innerHTML = formHtml({ error: `Save failed: ${apiStatusLabel(result)}` });
        attach();
        return;
      }
      closeAndReload(modal, `Cases saved (${cases.length})`);
    });
  }
  attach();
}

// ---- Precondition create -----------------------------------------------

export async function openPreconditionCreateModal(root, capability) {
  const formHtml = ({ error = "" } = {}) => `${errorBanner(error)}
    <form id="precondition-create-form" class="space-y-3">
      ${field("Name", "name", "", "e.g. integration-tests")}
      ${textarea("Shell command", "command", "", `curl -fsS http://localhost:8080/health | grep -q healthy`, 4)}
      ${field("Timeout (s)", "timeout_seconds", "60", "60", "number")}
    </form>`;
  const footer = `<button type="button" class="quiet-button" data-close-modal>Cancel</button>
    <button type="submit" form="precondition-create-form" class="primary-button">Create precondition</button>`;

  const modal = openModal({
    title: "New precondition",
    subtitle: `A shell command that must pass before a release activates for <span class="font-bold text-ink">${capability.name}</span>.`,
    body: formHtml(),
    footer,
    size: "narrow",
  });

  function attach() {
    modal.root.querySelector("#precondition-create-form")?.addEventListener("submit", async (event) => {
      event.preventDefault();
      const data = new FormData(event.target);
      const name = String(data.get("name") || "").trim();
      const command = String(data.get("command") || "").trim();
      const timeout = Number(data.get("timeout_seconds") || 60);
      if (!name || !command) {
        modal.body.innerHTML = formHtml({ error: "Name and command are required." });
        attach();
        return;
      }
      const result = await api.createPrecondition(capability.id, { name, command, timeout_seconds: timeout });
      if (!result.ok) {
        modal.body.innerHTML = formHtml({ error: `Could not create precondition: ${apiStatusLabel(result)}` });
        attach();
        return;
      }
      closeAndReload(modal, "Precondition created");
    });
  }
  attach();
}

// ---- Precondition edit ------------------------------------------------

export async function openPreconditionEditModal(root, id, existing) {
  const pre = existing || { id, name: "", command: "", timeout_seconds: 60 };
  const formHtml = ({ error = "" } = {}) => `${errorBanner(error)}
    <form id="precondition-edit-form" class="space-y-3">
      ${field("Name", "name", pre.name, "")}
      ${textarea("Shell command", "command", pre.command, "", 4)}
      ${field("Timeout (s)", "timeout_seconds", String(pre.timeout_seconds || 60), "", "number")}
    </form>`;
  const footer = `<button type="button" class="quiet-button" data-close-modal>Cancel</button>
    <button type="submit" form="precondition-edit-form" class="primary-button">Save</button>`;

  const modal = openModal({
    title: `Edit ${pre.name || pre.id}`,
    subtitle: pre.id,
    body: formHtml(),
    footer,
    size: "narrow",
  });

  function attach() {
    modal.root.querySelector("#precondition-edit-form")?.addEventListener("submit", async (event) => {
      event.preventDefault();
      const data = new FormData(event.target);
      const name = String(data.get("name") || "").trim();
      const command = String(data.get("command") || "").trim();
      const timeout = Number(data.get("timeout_seconds") || 60);
      if (!name || !command) {
        modal.body.innerHTML = formHtml({ error: "Name and command are required." });
        attach();
        return;
      }
      const result = await api.updatePrecondition(id, { name, command, timeout_seconds: timeout });
      if (!result.ok) {
        modal.body.innerHTML = formHtml({ error: `Could not save precondition: ${apiStatusLabel(result)}` });
        attach();
        return;
      }
      closeAndReload(modal, "Precondition saved");
    });
  }
  attach();
}
