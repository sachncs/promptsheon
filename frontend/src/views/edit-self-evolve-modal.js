// Edit self-evolve config modal. Triggered from the capability
// detail page's self-evolution panel. Sends PUT
// /api/v1/capabilities/{id}/self-evolve.

import { escape, apiStatusLabel } from "../utils.js";
import * as api from "../api.js";
import { openModal } from "../dialog.js";
import { inlineBanner } from "../ui.js";
import { toast } from "../toast.js";

function formHtml(cfg, capability, { error = "" } = {}) {
  return `${error ? inlineBanner({ tone: "danger", message: error }) : ""}
    <form id="se-form" class="space-y-4">
      <label class="flex items-center gap-3 rounded-xl border border-line p-3"><input type="checkbox" name="enabled" ${cfg.enabled ? "checked" : ""} class="h-4 w-4 accent-[#789c35]" autofocus /><span><span class="block text-[.72rem] font-bold">Enabled</span><span class="mt-0.5 block text-[.63rem] text-muted">When enabled, the orchestrator may auto-revise the prompt on score regressions.</span></span></label>
      <div class="grid gap-3 sm:grid-cols-2">
        <div><label class="field-label" for="se-min">Min score</label><input id="se-min" name="min_score" class="field mono" type="number" step="0.01" min="0" max="1" value="${escape(cfg.min_score ?? 0.9)}" /></div>
        <div><label class="field-label" for="se-rev">Max revisions</label><input id="se-rev" name="max_revisions" class="field mono" type="number" min="1" value="${escape(cfg.max_revisions ?? 10)}" /></div>
        <div><label class="field-label" for="se-cooldown">Cooldown (s)</label><input id="se-cooldown" name="cooldown_sec" class="field mono" type="number" min="0" value="${escape(cfg.cooldown_sec ?? 900)}" /></div>
        <div><label class="field-label" for="se-env">Target env</label><select id="se-env" name="target_env" class="field"><option value="dev" ${cfg.target_env === "dev" ? "selected" : ""}>dev</option><option value="staging" ${cfg.target_env === "staging" ? "selected" : ""}>staging</option><option value="prod" ${cfg.target_env === "prod" ? "selected" : ""}>prod</option></select></div>
        <div class="sm:col-span-2"><label class="field-label" for="se-ds">Dataset ID</label><input id="se-ds" name="dataset_id" class="field mono" value="${escape(cfg.dataset_id || "")}" placeholder="dataset id" /></div>
      </div>
    </form>`;
}

function footerHtml() {
  return `<button type="button" class="quiet-button" data-close-modal>Cancel</button>
    <button type="submit" form="se-form" class="primary-button">Save</button>`;
}

export async function openEditSelfEvolveModal(root, capability) {
  if (!root) return;
  const cfg = capability.self_evolve || {};
  const modal = openModal({
    title: capability.name,
    subtitle: "Closed-loop config; revisions are validated against the configured dataset before promotion.",
    body: formHtml(cfg, capability),
    footer: footerHtml(),
    size: "narrow",
  });

  function attach() {
    modal.root.querySelector("#se-form")?.addEventListener("submit", async (event) => {
      event.preventDefault();
      const data = new FormData(event.target);
      const payload = {
        enabled: data.get("enabled") === "on",
        min_score: Number(data.get("min_score") || 0),
        max_revisions: Number(data.get("max_revisions") || 0),
        cooldown_sec: Number(data.get("cooldown_sec") || 0),
        target_env: (data.get("target_env") || "dev").toString(),
        dataset_id: (data.get("dataset_id") || "").toString(),
      };
      const result = await api.updateSelfEvolveConfig(capability.id, payload);
      if (!result.ok) {
        modal.body.innerHTML = formHtml(cfg, capability, { error: apiStatusLabel(result) });
        attach();
        return;
      }
      modal.close();
      toast.success("Self-evolution config saved", capability.name);
      setTimeout(() => window.location.reload(), 250);
    });
  }
  attach();
}
