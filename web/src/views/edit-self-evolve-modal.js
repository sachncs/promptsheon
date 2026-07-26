import { escape, apiStatusLabel } from "../utils.js";
import * as api from "../api.js";

export async function openEditSelfEvolveModal(root, capability) {
  if (!root) return;
  const cfg = capability.self_evolve || {};
  const render = ({ error = null } = {}) => `<div class="modal-backdrop" role="presentation">
    <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="se-title">
      <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
        <div><div class="eyebrow">Self-evolution / Configure</div><h2 id="se-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">${escape(capability.name)}</h2><p class="mt-1 text-[.7rem] text-muted">Closed-loop config; revisions are validated against the configured dataset before promotion.</p></div>
        <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
      </div>
      <form id="se-form" class="space-y-4 px-5 py-5 sm:px-6">
        <label class="flex items-center gap-3 rounded-xl border border-line p-3"><input type="checkbox" name="enabled" ${cfg.enabled ? "checked" : ""} class="h-4 w-4 accent-[#789c35]" data-autofocus /><span><span class="block text-[.72rem] font-bold">Enabled</span><span class="mt-0.5 block text-[.63rem] text-muted">When enabled, the orchestrator may auto-revise the prompt on score regressions.</span></span></label>
        <div class="grid gap-3 sm:grid-cols-2">
          <div><label class="eyebrow mb-2 block" for="se-min">Min score</label><input id="se-min" name="min_score" class="field mono" type="number" step="0.01" min="0" max="1" value="${escape(cfg.min_score ?? 0.9)}" /></div>
          <div><label class="eyebrow mb-2 block" for="se-rev">Max revisions</label><input id="se-rev" name="max_revisions" class="field mono" type="number" min="1" value="${escape(cfg.max_revisions ?? 10)}" /></div>
          <div><label class="eyebrow mb-2 block" for="se-cooldown">Cooldown (s)</label><input id="se-cooldown" name="cooldown_sec" class="field mono" type="number" min="0" value="${escape(cfg.cooldown_sec ?? 900)}" /></div>
          <div><label class="eyebrow mb-2 block" for="se-env">Target env</label><select id="se-env" name="target_env" class="field"><option value="dev" ${cfg.target_env === "dev" ? "selected" : ""}>dev</option><option value="staging" ${cfg.target_env === "staging" ? "selected" : ""}>staging</option><option value="prod" ${cfg.target_env === "prod" ? "selected" : ""}>prod</option></select></div>
          <div class="sm:col-span-2"><label class="eyebrow mb-2 block" for="se-ds">Dataset ID</label><input id="se-ds" name="dataset_id" class="field mono" value="${escape(cfg.dataset_id || "")}" placeholder="dataset id" /></div>
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
  root.querySelector("#se-form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const data = new FormData(event.target);
    const payload = {
      enabled: data.get("enabled") === "on",
      min_score: Number(data.get("min_score") || 0),
      max_revisions: Number(data.get("max_revisions") || 0),
      cooldown_sec: Number(data.get("cooldown_sec") || 0),
      target_env: (data.get("target_env") || "dev").toString(),
      dataset_id: (data.get("dataset_id") || "").toString()
    };
    const result = await api.updateSelfEvolveConfig(capability.id, payload);
    if (!result.ok) {
      root.innerHTML = render({ error: apiStatusLabel(result) });
      attach(root, render, capability);
      return;
    }
    window.location.reload();
  });
}
