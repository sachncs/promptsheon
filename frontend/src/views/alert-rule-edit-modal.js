// Alert-rule edit modal. Triggered from the operations → alerts
// tab "Edit" buttons. Sends PUT /api/v1/alerts/rules/{id} with
// the updated name/type/severity/threshold/duration/window.

import * as api from "../api.js";
import { escape, apiStatusLabel } from "../utils.js";
import { openModal } from "../dialog.js";
import { inlineBanner } from "../ui.js";
import { toast } from "../toast.js";

function formHtml(rule, opts = {}) {
  const initial = rule || {};
  const v = {
    name: opts.name ?? initial.name ?? "",
    type: opts.type ?? initial.type ?? "budget_overrun",
    severity: opts.severity ?? initial.severity ?? "medium",
    threshold: opts.threshold ?? initial.threshold ?? 0.9,
    duration_minutes: opts.duration_minutes ?? initial.duration_minutes ?? initial.duration ?? 60,
    window_minutes: opts.window_minutes ?? initial.window_minutes ?? initial.window ?? 5,
  };
  return `${opts.error ? inlineBanner({ tone: "danger", message: opts.error }) : ""}
    <form id="alert-rule-edit-form" class="space-y-3">
      <div><label class="field-label" for="are-name">Name</label><input id="are-name" name="name" class="field" required value="${escape(v.name)}" /></div>
      <div class="grid gap-3 sm:grid-cols-2">
        <div><label class="field-label" for="are-type">Type</label>
          <select id="are-type" name="type" class="field" required>
            <option value="budget_overrun" ${v.type === "budget_overrun" ? "selected" : ""}>Budget overrun</option>
            <option value="latency_p99" ${v.type === "latency_p99" ? "selected" : ""}>Latency p99</option>
            <option value="error_rate" ${v.type === "error_rate" ? "selected" : ""}>Error rate</option>
            <option value="queue_depth" ${v.type === "queue_depth" ? "selected" : ""}>Queue depth</option>
          </select>
        </div>
        <div><label class="field-label" for="are-severity">Severity</label>
          <select id="are-severity" name="severity" class="field" required>
            <option value="critical" ${v.severity === "critical" ? "selected" : ""}>critical</option>
            <option value="high" ${v.severity === "high" ? "selected" : ""}>high</option>
            <option value="medium" ${v.severity === "medium" ? "selected" : ""}>medium</option>
            <option value="low" ${v.severity === "low" ? "selected" : ""}>low</option>
          </select>
        </div>
      </div>
      <div class="grid gap-3 sm:grid-cols-3">
        <div><label class="field-label" for="are-threshold">Threshold</label><input id="are-threshold" name="threshold" type="number" step="0.01" class="field mono" required value="${escape(String(v.threshold))}" /></div>
        <div><label class="field-label" for="are-duration">Duration (min)</label><input id="are-duration" name="duration_minutes" type="number" class="field mono" required value="${escape(String(v.duration_minutes))}" /></div>
        <div><label class="field-label" for="are-window">Window (min)</label><input id="are-window" name="window_minutes" type="number" class="field mono" required value="${escape(String(v.window_minutes))}" /></div>
      </div>
    </form>`;
}

function footerHtml() {
  return `<button type="button" class="quiet-button" data-close-modal>Cancel</button>
    <button type="submit" form="alert-rule-edit-form" class="primary-button">Save</button>`;
}

export async function openAlertRuleEditModal(root, rule) {
  if (!rule) return;
  const modal = openModal({
    title: `Edit ${rule.name || rule.id}`,
    subtitle: rule.id,
    body: formHtml(rule),
    footer: footerHtml(),
    size: "wide",
  });

  function attach(currentValues = {}) {
    modal.root.querySelector("#alert-rule-edit-form")?.addEventListener("submit", async (event) => {
      event.preventDefault();
      const data = new FormData(event.target);
      const payload = {
        name: String(data.get("name") || "").trim(),
        type: String(data.get("type") || "").trim(),
        severity: String(data.get("severity") || "").trim(),
        threshold: Number(data.get("threshold") || 0),
        duration_minutes: Number(data.get("duration_minutes") || 0),
        window_minutes: Number(data.get("window_minutes") || 0),
      };
      if (!payload.name || !payload.type || !payload.severity) {
        modal.body.innerHTML = formHtml({ ...rule, ...currentValues }, { ...payload, error: "Name, type, and severity are required." });
        attach(payload);
        return;
      }
      const result = await api.updateAlertRule(rule.id, payload);
      if (!result.ok) {
        modal.body.innerHTML = formHtml({ ...rule, ...currentValues }, { ...payload, error: `Could not save: ${apiStatusLabel(result)}` });
        attach(payload);
        return;
      }
      modal.close();
      toast.success("Alert rule updated", rule.name || rule.id);
      setTimeout(() => window.location.reload(), 250);
    });
  }
  attach({});
}
