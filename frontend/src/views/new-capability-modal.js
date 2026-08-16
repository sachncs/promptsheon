// New capability modal. Triggered from the capability list's
// New button, the overview's New capability action, and any
// project detail page. Lets the operator pick a project, name the
// capability, set owner / tags, and optionally seed the first
// version + release.

import * as api from "../api.js";
import { apiStatusLabel } from "../utils.js";
import { openModal } from "../dialog.js";
import { inlineBanner, statusPill } from "../ui.js";
import { toast } from "../toast.js";
import { openProjectCreateModal } from "./project-create-modal.js";

function optional(label, value) {
  return `<div class="rounded-lg bg-paper p-3"><span class="eyebrow">${label}</span><span class="mono mt-2 block text-[.78rem] font-bold">${value}</span></div>`;
}

function createdBlock(created) {
  return `<div class="rounded-xl border border-line bg-paper p-3">
    <div class="flex items-center justify-between"><div><p class="text-[.7rem] font-bold">Created</p><p class="mt-1 text-[.62rem] text-muted mono">${created.id}</p></div>${created.environment ? statusPill(`${created.environment} pending`, "warn") : ""}</div>
    <div class="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-4">
      ${optional("Capability", created.id)}
      ${created.versionId ? optional("Version", created.versionId) : ""}
      ${created.releaseId ? optional("Release", created.releaseId) : ""}
      ${created.environment ? optional("Env", created.environment) : ""}
    </div>
  </div>`;
}

function formHtml(state, { error = "" } = {}) {
  const { projects, users, autofocus = true, created = null } = state;
  const options = (projects || []).map((p) => `<option value="${p.id}"${p.selected ? " selected" : ""}>${p.name}</option>`).join("");
  const noProjects = !projects || projects.length === 0;
  const ownerOptions = (users || []).map((u) => `<option value="${u.id}">${u.name || u.email || u.id}</option>`).join("");
  return `${error ? inlineBanner({ tone: "danger", message: error }) : ""}
    <form id="new-capability-form" class="space-y-5">
      <div><label class="field-label" for="nc-name">Capability name</label><input id="nc-name" name="name" class="field" required ${autofocus ? "autofocus" : ""} placeholder="e.g. Summarize customer feedback" /></div>
      <div><label class="field-label" for="nc-description">Business outcome</label><textarea id="nc-description" name="description" class="field min-h-24 resize-y" placeholder="What should this capability reliably help your team accomplish?"></textarea></div>
      <div class="grid gap-4 sm:grid-cols-2">
        <div>
          <label class="field-label" for="nc-project">Project</label>
          <select id="nc-project" name="project_id" class="field" required>
            ${options || (noProjects ? `<option value="" disabled selected>No projects yet — create one first</option>` : `<option value="" disabled selected>Select a project</option>`)}
          </select>
        </div>
        <div><label class="field-label" for="nc-owner">Owner</label><select id="nc-owner" name="owner" class="field"><option value="">— unassigned —</option>${ownerOptions}</select></div>
      </div>
      <div><label class="field-label" for="nc-tags">Tags (comma separated)</label><input id="nc-tags" name="tags" class="field" placeholder="research, finance" /></div>
      <details class="rounded-xl border border-line bg-paper/40 p-3 text-[.74rem]">
        <summary class="cursor-pointer select-none font-bold text-ink">Advanced — create the first version and a pending release in one shot</summary>
        <div class="mt-3 grid gap-3 sm:grid-cols-3">
          <div><label class="field-label" for="nc-env">Target environment</label><select id="nc-env" name="environment" class="field"><option value="">— skip —</option><option value="dev">dev</option><option value="staging">staging</option><option value="prod">prod</option></select></div>
          <div><label class="field-label" for="nc-prompt">Prompt</label><input id="nc-prompt" name="prompt" class="field mono" placeholder="System prompt text" /></div>
          <div><label class="field-label" for="nc-model">Model</label><input id="nc-model" name="model" class="field mono" placeholder="gpt-4o-mini" /></div>
        </div>
        <p class="mt-3 text-[.66rem] text-muted">Hashes are computed client-side from the prompt and model strings.</p>
      </details>
      ${created ? createdBlock(created) : ""}
    </form>`;
}

function footerHtml() {
  return `<button type="button" class="quiet-button" data-close-modal>Cancel</button>
    <button type="submit" form="new-capability-form" class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>Create capability</button>`;
}

export async function openNewCapabilityModal(root, projects) {
  if (!root) return;
  const users = (await api.listUsers(50)).data || [];
  const workspaces = (await api.listWorkspaces()).data || [];

  // Pre-select a project when the caller hands us a single one
  // (project-detail page pattern).
  const initialState = {
    projects: (projects || []).map((p, idx) => ({ ...p, selected: idx === 0 })),
    users,
    autofocus: true,
  };

  if (!initialState.projects.length) {
    const errorMessage = workspaces.length === 0
      ? "No workspaces exist yet."
      : "No projects in this workspace yet.";
    const modal = openModal({
      title: "Create a capability",
      subtitle: "Define the outcome first. Implementation can evolve behind it.",
      body: formHtml(initialState, { error: errorMessage }),
      footer: `<button type="button" class="quiet-button" data-close-modal>Cancel</button>
        <button type="button" id="new-cap-create-project" class="quiet-button">Create project</button>`,
      size: "wide",
    });
    modal.root.querySelector("#new-cap-create-project")?.addEventListener("click", () => {
      openProjectCreateModal(modal.root, { workspaces, onCreated: () => window.location.reload() });
    });
    return;
  }

  const modal = openModal({
    title: "Create a capability",
    subtitle: "Define the outcome first. Implementation can evolve behind it.",
    body: formHtml(initialState),
    footer: footerHtml(),
    size: "wide",
  });

  function attach() {
    modal.root.querySelector("#new-capability-form")?.addEventListener("submit", async (event) => {
      event.preventDefault();
      const form = event.target;
      const data = new FormData(form);
      const projectId = (data.get("project_id") || "").toString();
      const tags = (data.get("tags") || "").toString().split(",").map((s) => s.trim()).filter(Boolean);
      const payload = {
        name: (data.get("name") || "").toString().trim(),
        description: (data.get("description") || "").toString().trim() || undefined,
        owner: (data.get("owner") || "").toString() || undefined,
        tags: tags.length ? tags : undefined,
      };
      const environment = (data.get("environment") || "").toString();
      if (!projectId) {
        modal.body.innerHTML = formHtml(initialState, { error: "Pick a project first." });
        attach();
        return;
      }
      const submitButton = modal.root.querySelector('.modal-footer button[type="submit"]');
      if (submitButton) submitButton.disabled = true;
      const capRes = await api.createCapability(projectId, payload);
      if (!capRes.ok) {
        if (submitButton) submitButton.disabled = false;
        modal.body.innerHTML = formHtml(initialState, { error: apiStatusLabel(capRes) });
        attach();
        return;
      }
      const created = { id: capRes.data.id };
      if (environment) {
        const prompt = (data.get("prompt") || "").toString().trim() || `You are a ${payload.name.toLowerCase()} assistant.`;
        const model = (data.get("model") || "").toString().trim() || "gpt-4o-mini";
        const versionRes = await api.createVersion(capRes.data.id, {
          version: 1,
          manifest: {
            prompt: { kind: "prompt", hash: await sha256Hex(prompt) },
            model_policy: { kind: "model_policy", hash: await sha256Hex(JSON.stringify({ model })) },
            runtime_policy: { kind: "runtime_policy", hash: await sha256Hex(JSON.stringify({ max_tokens: 600, timeout_sec: 30 })) },
            context_contract: { kind: "context_contract", hash: await sha256Hex(JSON.stringify({ max_input_tokens: 12000 })) },
            memory: { kind: "memory", hash: await sha256Hex(JSON.stringify({ kind: "ephemeral" })) },
          },
        });
        if (!versionRes.ok) {
          if (submitButton) submitButton.disabled = false;
          modal.body.innerHTML = formHtml(initialState, { error: `Created capability but version failed: ${apiStatusLabel(versionRes)}` });
          attach();
          return;
        }
        created.versionId = versionRes.data.id;
        const releaseRes = await api.getReleaseCreation(versionRes.data.id, environment);
        if (!releaseRes.ok) {
          if (submitButton) submitButton.disabled = false;
          modal.body.innerHTML = formHtml(initialState, { error: `Created capability+version but release failed: ${apiStatusLabel(releaseRes)}` });
          attach();
          return;
        }
        created.releaseId = releaseRes.data.id;
        created.environment = environment;
      }
      modal.close();
      toast.success("Capability created", created.id);
      setTimeout(() => window.location.reload(), 250);
    });
  }
  attach();
}

async function sha256Hex(input) {
  const bytes = new TextEncoder().encode(input);
  const hash = await crypto.subtle.digest("SHA-256", bytes);
  return Array.from(new Uint8Array(hash)).map((b) => b.toString(16).padStart(2, "0")).join("");
}
