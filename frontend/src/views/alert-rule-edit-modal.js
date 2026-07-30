// Alert-rule edit modal. Triggered from the operations → alerts
// tab "Edit" buttons. Sends PUT /api/v1/alerts/rules/{id} with
// the updated name/type/severity/threshold/duration/window.
import * as api from "../api.js";
import { escape, apiStatusLabel } from "../utils.js";

function showError(slot, message) {
  slot.textContent = message;
  slot.classList.remove("hidden");
}

export async function openAlertRuleEditModal(root, rule) {
  if (!rule) return;
  const id = rule.id;
  const initialName = rule.name || "";
  const initialType = rule.type || "budget_overrun";
  const initialSeverity = rule.severity || "medium";
  const initialThreshold = rule.threshold ?? 0.9;
  const initialDuration = rule.duration_minutes ?? rule.duration ?? 60;
  const initialWindow = rule.window_minutes ?? rule.window ?? 5;

  function render(opts = {}) {
    return `<div class="modal-backdrop" role="presentation">
      <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="are-title">
        <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
          <div>
            <div class="eyebrow">Alert rule</div>
            <h2 id="are-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">Edit ${escape(initialName || id)}</h2>
            <p class="mt-1 text-[.7rem] text-muted mono">${escape(id)}</p>
          </div>
          <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
        </div>
        <form id="alert-rule-edit-form" class="space-y-3 px-5 py-5 sm:px-6">
          <p id="alert-rule-edit-error" class="hidden rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800"></p>
          <div><label class="eyebrow mb-2 block" for="are-name">Name</label><input id="are-name" name="name" class="field" required value="${escape(opts.name ?? initialName)}" /></div>
          <div class="grid gap-3 sm:grid-cols-2">
            <div><label class="eyebrow mb-2 block" for="are-type">Type</label>
              <select id="are-type" name="type" class="field" required>
                <option value="budget_overrun" ${(opts.type ?? initialType) === "budget_overrun" ? "selected" : ""}>Budget overrun</option>
                <option value="latency_p99" ${(opts.type ?? initialType) === "latency_p99" ? "selected" : ""}>Latency p99</option>
                <option value="error_rate" ${(opts.type ?? initialType) === "error_rate" ? "selected" : ""}>Error rate</option>
                <option value="queue_depth" ${(opts.type ?? initialType) === "queue_depth" ? "selected" : ""}>Queue depth</option>
              </select>
            </div>
            <div><label class="eyebrow mb-2 block" for="are-severity">Severity</label>
              <select id="are-severity" name="severity" class="field" required>
                <option value="critical" ${(opts.severity ?? initialSeverity) === "critical" ? "selected" : ""}>critical</option>
                <option value="high" ${(opts.severity ?? initialSeverity) === "high" ? "selected" : ""}>high</option>
                <option value="medium" ${(opts.severity ?? initialSeverity) === "medium" ? "selected" : ""}>medium</option>
                <option value="low" ${(opts.severity ?? initialSeverity) === "low" ? "selected" : ""}>low</option>
              </select>
            </div>
          </div>
          <div class="grid gap-3 sm:grid-cols-3">
            <div><label class="eyebrow mb-2 block" for="are-threshold">Threshold</label><input id="are-threshold" name="threshold" type="number" step="0.01" class="field mono" required value="${escape(String(opts.threshold ?? initialThreshold))}" /></div>
            <div><label class="eyebrow mb-2 block" for="are-duration">Duration (min)</label><input id="are-duration" name="duration_minutes" type="number" class="field mono" required value="${escape(String(opts.duration_minutes ?? initialDuration))}" /></div>
            <div><label class="eyebrow mb-2 block" for="are-window">Window (min)</label><input id="are-window" name="window_minutes" type="number" class="field mono" required value="${escape(String(opts.window_minutes ?? initialWindow))}" /></div>
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
  root.querySelector("#alert-rule-edit-form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const data = new FormData(event.target);
    const payload = {
      name: String(data.get("name") || "").trim(),
      type: String(data.get("type") || "").trim(),
      severity: String(data.get("severity") || "").trim(),
      threshold: Number(data.get("threshold") || 0),
      duration_minutes: Number(data.get("duration_minutes") || 0),
      window_minutes: Number(data.get("window_minutes") || 0)
    };
    const errSlot = root.querySelector("#alert-rule-edit-error");
    errSlot.classList.add("hidden");
    if (!payload.name || !payload.type || !payload.severity) {
      showError(errSlot, "Name, type, and severity are required.");
      return;
    }
    const result = await api.updateAlertRule(id, payload);
    if (!result.ok) {
      root.innerHTML = render({ ...payload, error: `Could not save: ${escape(apiStatusLabel(result))}` });
      openAlertRuleEditModal(root, { id, ...payload });
      return;
    }
    window.location.reload();
  });
}