import * as api from "../api.js";
import { escape, apiStatusLabel } from "../utils.js";

function optional(label, value) {
  return `<div class="rounded-lg bg-paper p-3"><span class="eyebrow">${escape(label)}</span><span class="mono mt-2 block text-[.78rem] font-bold">${escape(value)}</span></div>`;
}

export async function openNewCapabilityModal(root, projects) {
  if (!root) return;
  const render = (opts) => {
    const noProjects = !opts.projects || opts.projects.length === 0;
    const options = (opts.projects || []).map((p) => `<option value="${escape(p.id)}"${p.selected ? " selected" : ""}>${escape(p.name)}</option>`).join("");
    return `<div class="modal-backdrop" role="presentation">
      <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="new-capability-title">
        <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
          <div><div class="eyebrow">Catalog / New</div><h2 id="new-capability-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">Create a capability</h2><p class="mt-1 text-[.7rem] text-muted">Define the outcome first. Implementation can evolve behind it.</p></div>
          <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
        </div>
        <form id="new-capability-form" class="space-y-5 px-5 py-5 sm:px-6">
          <div><label class="eyebrow mb-2 block" for="nc-name">Capability name</label><input id="nc-name" name="name" class="field" required ${opts.autofocus ? "data-autofocus" : ""} placeholder="e.g. Summarize customer feedback" /></div>
          <div><label class="eyebrow mb-2 block" for="nc-description">Business outcome</label><textarea id="nc-description" name="description" class="field min-h-24 resize-y" placeholder="What should this capability reliably help your team accomplish?"></textarea></div>
          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <label class="eyebrow mb-2 block" for="nc-project">Project</label>
              <select id="nc-project" name="project_id" class="field" required>
                ${options || (noProjects ? `<option value="" disabled selected>No projects yet — create one first</option>` : `<option value="" disabled selected>Select a project</option>`)}
              </select>
            </div>
            <div><label class="eyebrow mb-2 block" for="nc-owner">Owner</label><select id="nc-owner" name="owner" class="field"><option value="">— unassigned —</option>${(opts.users || []).map((u) => `<option value="${escape(u.id)}">${escape(u.name || u.email || u.id)}</option>`).join("")}</select></div>
          </div>
          <div><label class="eyebrow mb-2 block" for="nc-tags">Tags (comma separated)</label><input id="nc-tags" name="tags" class="field" placeholder="research, finance" /></div>
          <details class="rounded-xl border border-line bg-paper/40 p-3 text-[.74rem]">
            <summary class="cursor-pointer select-none font-bold text-ink">Advanced — create the first version and a pending release in one shot</summary>
            <div class="mt-3 grid gap-3 sm:grid-cols-3">
              <div><label class="eyebrow mb-2 block" for="nc-env">Target environment</label><select id="nc-env" name="environment" class="field"><option value="">— skip —</option><option value="dev">dev</option><option value="staging">staging</option><option value="prod">prod</option></select></div>
              <div><label class="eyebrow mb-2 block" for="nc-prompt">Prompt</label><input id="nc-prompt" name="prompt" class="field mono" placeholder="System prompt text" /></div>
              <div><label class="eyebrow mb-2 block" for="nc-model">Model</label><input id="nc-model" name="model" class="field mono" placeholder="gpt-4o-mini" /></div>
            </div>
            <p class="mt-3 text-[.66rem] text-muted">Hashes are computed client-side from the prompt and model strings.</p>
          </details>
          <p id="nc-error" class="hidden rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800"></p>
          ${opts.error ? `<p class="rounded-lg bg-amber-50 px-3 py-2 text-[.68rem] text-amber-800">${escape(opts.error)}</p>` : ""}
          ${opts.created ? renderCreated(opts.created) : ""}
          <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
            <button type="button" class="quiet-button" data-close-modal>Cancel</button>
            <button type="submit" class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>Create capability</button>
          </div>
        </form>
      </section>
    </div>`;
  };

  const users = (await api.listUsers(50)).data || [];
  if (!projects?.length) {
    root.innerHTML = render({
      projects,
      users,
      autofocus: true,
      error: "No projects in this workspace yet. Create one with `POST /api/v1/workspaces/{id}/projects` first, then reload."
    });
    attach(root, render, { projects, users });
    return;
  }
  // Highlight the preselect in the error if nothing matches
  root.innerHTML = render({ projects, users, autofocus: true });
  attach(root, render, { projects, users });
}

function renderCreated(created) {
  return `<div class="rounded-xl border border-line bg-paper p-3">
    <div class="flex items-center justify-between"><div><p class="text-[.7rem] font-bold">Created</p><p class="mt-1 text-[.62rem] text-muted mono">${escape(created.id)}</p></div>${created.environment ? `<span class="status-pill warn !px-2 !py-1">${escape(created.environment)} pending</span>` : ""}</div>
    <div class="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-4">
      ${optional("Capability", created.id)}
      ${created.versionId ? optional("Version", created.versionId) : ""}
      ${created.releaseId ? optional("Release", created.releaseId) : ""}
      ${created.environment ? optional("Env", created.environment) : ""}
    </div>
  </div>`;
}

function attach(root, render, state) {
  const form = root.querySelector("#new-capability-form");
  form?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const data = new FormData(form);
    const projectId = (data.get("project_id") || "").toString();
    const tags = (data.get("tags") || "").toString().split(",").map((s) => s.trim()).filter(Boolean);
    const payload = {
      name: (data.get("name") || "").toString().trim(),
      description: (data.get("description") || "").toString().trim() || undefined,
      owner: (data.get("owner") || "").toString() || undefined,
      tags: tags.length ? tags : undefined
    };
    const environment = (data.get("environment") || "").toString();
    if (!projectId) {
      return showError(root, render, state, null, "Pick a project first.");
    }
    const submitButton = form.querySelector("button[type=submit]");
    if (submitButton) submitButton.disabled = true;
    const capRes = await api.createCapability(projectId, payload);
    if (!capRes.ok) {
      if (submitButton) submitButton.disabled = false;
      return showError(root, render, state, capRes, apiStatusLabel(capRes));
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
          memory: { kind: "memory", hash: await sha256Hex(JSON.stringify({ kind: "ephemeral" })) }
        }
      });
      if (!versionRes.ok) {
        if (submitButton) submitButton.disabled = false;
        return showError(root, render, state, versionRes, `Created capability but version failed: ${apiStatusLabel(versionRes)}`);
      }
      created.versionId = versionRes.data.id;
      const releaseRes = await api.getReleaseCreation(versionRes.data.id, environment);
      if (!releaseRes.ok) {
        if (submitButton) submitButton.disabled = false;
        return showError(root, render, state, releaseRes, `Created capability+version but release failed: ${apiStatusLabel(releaseRes)}`);
      }
      created.releaseId = releaseRes.data.id;
      created.environment = environment;
    }
    if (submitButton) submitButton.disabled = false;
    root.innerHTML = render({ projects: state.projects, users: state.users, autofocus: false, created });
    attach(root, render, state);
    window.location.reload();
  });
  root.querySelector("[data-close-modal]")?.addEventListener("click", () => root.replaceChildren());
}

function showError(root, render, state, result, message) {
  root.innerHTML = render({ projects: state.projects, users: state.users, autofocus: false, error: message });
  attach(root, render, state);
}

async function sha256Hex(input) {
  const bytes = new TextEncoder().encode(input);
  const hash = await crypto.subtle.digest("SHA-256", bytes);
  return Array.from(new Uint8Array(hash)).map((b) => b.toString(16).padStart(2, "0")).join("");
}
