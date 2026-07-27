import { escape, apiStatusLabel } from "../utils.js";
import * as api from "../api.js";

export async function openNewVersionModal(root, capability, existingVersions) {
  if (!root) return;
  const nextVersion = (existingVersions?.reduce((m, v) => Math.max(m, v.version || 0), 0) || 0) + 1;
  const render = ({ error = null, created = null } = {}) => `<div class="modal-backdrop" role="presentation">
    <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="new-version-title">
      <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
        <div><div class="eyebrow">Versions / New</div><h2 id="new-version-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">New version for ${escape(capability.name)}</h2><p class="mt-1 text-[.7rem] text-muted">Manifest artifacts are sha256 hashes; paste them in or compute via the helper.</p></div>
        <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
      </div>
      <form id="new-version-form" class="space-y-4 px-5 py-5 sm:px-6">
        <div><label class="eyebrow mb-2 block" for="nv-version">Version number</label><input id="nv-version" name="version" class="field mono" required type="number" min="1" value="${escape(nextVersion)}" data-autofocus /></div>
        <div class="grid gap-3 sm:grid-cols-2">
          ${hashField("nv-prompt", "Prompt hash", "prompt")}
          ${hashField("nv-model", "Model policy hash", "model_policy")}
          ${hashField("nv-runtime", "Runtime policy hash", "runtime_policy")}
          ${hashField("nv-context", "Context contract hash", "context_contract")}
          ${hashField("nv-memory", "Memory hash", "memory")}
        </div>
        ${error ? `<p class="rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800">${escape(error)}</p>` : ""}
        ${created ? `<p class="rounded-lg bg-lime/15 px-3 py-2 text-[.68rem] text-[#52632d]">Created <span class="mono">${escape(created.id)}</span>.</p>` : ""}
        <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
          <button type="button" class="quiet-button" data-close-modal>Cancel</button>
          <button type="submit" class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>Create version</button>
        </div>
      </form>
    </section>
  </div>`;
  root.innerHTML = render();
  attach(root, render, capability);
}

function hashField(id, label, kind) {
  return `<div><label class="eyebrow mb-2 block" for="${id}">${escape(label)}</label><div class="flex gap-1"><input id="${id}" name="${kind}" class="field mono" required placeholder="64 hex chars" /><button type="button" data-fill-hash="${kind}" class="quiet-button !h-9 !px-2 !text-[.66rem]">Compute</button></div></div>`;
}

function attach(root, render, capability) {
  root.querySelector("[data-close-modal]")?.addEventListener("click", () => root.replaceChildren());
  root.querySelectorAll("[data-fill-hash]").forEach((b) => {
    b.addEventListener("click", async () => {
      const kind = b.dataset.fillHash;
      const input = root.querySelector(`#nv-${kind === "prompt" ? "prompt" : kind === "model_policy" ? "model" : kind === "runtime_policy" ? "runtime" : kind === "context_contract" ? "context" : "memory"}`);
      const txt = window.prompt(`Paste the source text for the ${kind} artifact to hash:`);
      if (!txt) return;
      const hash = await sha256Hex(txt);
      if (input) input.value = hash;
    });
  });
  root.querySelector("#new-version-form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const data = new FormData(event.target);
    const payload = {
      version: Number(data.get("version")),
      manifest: {
        prompt: { kind: "prompt", hash: (data.get("prompt") || "").toString() },
        model_policy: { kind: "model_policy", hash: (data.get("model_policy") || "").toString() },
        runtime_policy: { kind: "runtime_policy", hash: (data.get("runtime_policy") || "").toString() },
        context_contract: { kind: "context_contract", hash: (data.get("context_contract") || "").toString() },
        memory: { kind: "memory", hash: (data.get("memory") || "").toString() }
      }
    };
    const result = await api.createVersion(capability.id, payload);
    if (!result.ok) {
      root.innerHTML = render({ error: apiStatusLabel(result) });
      attach(root, render, capability);
      return;
    }
    window.location.reload();
  });
}

async function sha256Hex(input) {
  const bytes = new TextEncoder().encode(input);
  const hash = await crypto.subtle.digest("SHA-256", bytes);
  return Array.from(new Uint8Array(hash)).map((b) => b.toString(16).padStart(2, "0")).join("");
}
