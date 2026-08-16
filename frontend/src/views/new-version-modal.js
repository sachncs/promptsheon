// New-version modal. Triggered from the capability detail page's
// Versions panel. Sends POST /api/v1/capabilities/{id}/versions with
// the manifest artifact hashes.

import { escape, apiStatusLabel } from "../utils.js";
import * as api from "../api.js";
import { openModal } from "../dialog.js";
import { inlineBanner } from "../ui.js";
import { toast } from "../toast.js";

const ARTIFACT_FIELDS = [
  { id: "nv-prompt", label: "Prompt hash", kind: "prompt" },
  { id: "nv-model", label: "Model policy hash", kind: "model_policy" },
  { id: "nv-runtime", label: "Runtime policy hash", kind: "runtime_policy" },
  { id: "nv-context", label: "Context contract hash", kind: "context_contract" },
  { id: "nv-memory", label: "Memory hash", kind: "memory" },
];

function formHtml(capability, nextVersion, { error = "" } = {}) {
  const hashFields = ARTIFACT_FIELDS.map((f) => `
    <div>
      <label class="field-label" for="${f.id}">${f.label}</label>
      <div class="flex gap-1">
        <input id="${f.id}" name="${f.kind}" class="field mono" required placeholder="64 hex chars" />
        <button type="button" data-fill-hash="${f.kind}" class="quiet-button !h-9 !px-2 !text-[.66rem]">Compute</button>
      </div>
    </div>
  `).join("");
  return `${error ? inlineBanner({ tone: "danger", message: error }) : ""}
    <form id="new-version-form" class="space-y-4">
      <div><label class="field-label" for="nv-version">Version number</label><input id="nv-version" name="version" class="field mono" required type="number" min="1" value="${escape(nextVersion)}" autofocus /></div>
      <div class="grid gap-3 sm:grid-cols-2">${hashFields}</div>
    </form>`;
}

function footerHtml() {
  return `<button type="button" class="quiet-button" data-close-modal>Cancel</button>
    <button type="submit" form="new-version-form" class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>Create version</button>`;
}

export async function openNewVersionModal(root, capability, existingVersions) {
  if (!root) return;
  const nextVersion = (existingVersions?.reduce((m, v) => Math.max(m, v.version || 0), 0) || 0) + 1;
  const modal = openModal({
    title: `New version for ${capability.name}`,
    subtitle: "Manifest artifacts are sha256 hashes; paste them in or compute via the helper.",
    body: formHtml(capability, nextVersion),
    footer: footerHtml(),
    size: "wide",
  });

  function attach() {
    modal.root.querySelectorAll("[data-fill-hash]").forEach((b) => {
      b.addEventListener("click", async () => {
        const kind = b.dataset.fillHash;
        const input = modal.root.querySelector(`#nv-${kind === "prompt" ? "prompt" : kind === "model_policy" ? "model" : kind === "runtime_policy" ? "runtime" : kind === "context_contract" ? "context" : "memory"}`);
        const txt = window.prompt(`Paste the source text for the ${kind} artifact to hash:`);
        if (!txt) return;
        const hash = await sha256Hex(txt);
        if (input) input.value = hash;
      });
    });
    modal.root.querySelector("#new-version-form")?.addEventListener("submit", async (event) => {
      event.preventDefault();
      const data = new FormData(event.target);
      const payload = {
        version: Number(data.get("version")),
        manifest: {
          prompt: { kind: "prompt", hash: (data.get("prompt") || "").toString() },
          model_policy: { kind: "model_policy", hash: (data.get("model_policy") || "").toString() },
          runtime_policy: { kind: "runtime_policy", hash: (data.get("runtime_policy") || "").toString() },
          context_contract: { kind: "context_contract", hash: (data.get("context_contract") || "").toString() },
          memory: { kind: "memory", hash: (data.get("memory") || "").toString() },
        },
      };
      const result = await api.createVersion(capability.id, payload);
      if (!result.ok) {
        modal.body.innerHTML = formHtml(capability, nextVersion, { error: apiStatusLabel(result) });
        attach();
        return;
      }
      modal.close();
      toast.success("Version created", `v${payload.version}`);
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
