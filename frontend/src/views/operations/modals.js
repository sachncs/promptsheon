// Operations-area create modals: alert rule, notification group,
// webhook, vault key, user. Each one routes through the shared
// dialog system so the title / body / footer / cancel / close / ESC
// / focus-trap behavior matches every other dashboard modal.

import * as api from "../../api.js";
import { apiStatusLabel } from "../../utils.js";
import { openModal } from "../../dialog.js";
import { inlineBanner } from "../../ui.js";
import { toast } from "../../toast.js";

const OPERATIONS = {
  rule: async (data) => {
    const payload = {
      name: (data.get("name") || "").toString().trim(),
      type: (data.get("type") || "threshold").toString(),
      severity: (data.get("severity") || "medium").toString(),
      threshold: Number(data.get("threshold") || 0),
      duration_minutes: Number(data.get("duration_minutes") || 0),
      window_minutes: Number(data.get("window_minutes") || 0),
      enabled: data.get("enabled") === "on",
    };
    return { payload, result: await api.createAlertRule(payload) };
  },
  group: async (data) => {
    const payload = {
      name: (data.get("name") || "").toString().trim(),
      channels: (data.get("channels") || "log").toString().split(",").map((s) => s.trim()).filter(Boolean),
    };
    return { payload, result: await api.createNotificationGroup(payload) };
  },
  webhook: async (data) => {
    const payload = {
      url: (data.get("url") || "").toString().trim(),
      events: (data.get("events") || "").toString().split(",").map((s) => s.trim()).filter(Boolean),
      secret: (data.get("secret") || "").toString(),
    };
    return { payload, result: await api.createWebhook(payload) };
  },
  vault: async (data) => {
    const payload = {
      provider_name: (data.get("provider_name") || "").toString().trim(),
      key_name: (data.get("key_name") || "").toString().trim(),
      key: (data.get("key") || "").toString(),
    };
    return { payload, result: await api.saveVaultKey(payload) };
  },
  user: async (data) => {
    const payload = {
      email: (data.get("email") || "").toString().trim(),
      name: (data.get("name") || "").toString().trim(),
      role: (data.get("role") || "reader").toString(),
    };
    return { payload, result: await api.createUser(payload) };
  },
};

const SUCCESS_MESSAGES = {
  rule: "Alert rule created",
  group: "Notification group created",
  webhook: "Webhook created",
  vault: "Vault key saved",
  user: "User created",
};

function createOpener({ op, title, subtitle, formInner }) {
  return async function open(root, refresh) {
    if (!root) return;
    const bodyHtml = `${inlineBanner({ tone: "info", message: subtitle })}
      <form id="op-form" class="space-y-3">
        ${formInner}
      </form>`;
    const modal = openModal({
      title,
      subtitle,
      body: bodyHtml,
      footer: `<button type="button" class="quiet-button" data-close-modal>Cancel</button>
        <button type="submit" form="op-form" class="primary-button">Save</button>`,
      size: "wide",
    });
    attach(op, modal, refresh);
  };
}

function attach(op, modal, refresh) {
  const root = modal.root;
  const handler = async (event) => {
    event.preventDefault();
    const data = new FormData(event.target);
    const submit = root.querySelector('button[type="submit"]');
    if (submit) submit.disabled = true;
    let payload = null;
    let result = null;
    let errMessage = "";
    try {
      ({ payload, result } = await OPERATIONS[op](data));
    } catch (e) {
      errMessage = String(e?.message || e);
    }
    if (submit) submit.disabled = false;
    if (errMessage || !result || !result.ok) {
      const msg = errMessage || apiStatusLabel(result || { error: "no response" });
      const errorSlot = root.querySelector(".op-modal-error");
      if (errorSlot) {
        errorSlot.innerHTML = inlineBanner({ tone: "danger", message: msg });
        errorSlot.classList.remove("hidden");
      }
      return;
    }
    modal.close();
    toast.success(SUCCESS_MESSAGES[op] || "Created", payload?.name || "");
      setTimeout(() => { if (typeof refresh === "function") refresh(); }, 200);
  };
  const form = root.querySelector("#op-form");
  form?.addEventListener("submit", handler);
}

export const openNewRuleModal = createOpener({
  op: "rule",
  title: "New alert rule",
  subtitle: "Threshold + duration + window define when the rule fires.",
  formInner: `
    <input type="hidden" name="op" value="rule" />
    <div><label class="field-label" for="ar-name">Name</label><input id="ar-name" name="name" class="field" required autofocus /></div>
    <div class="grid gap-3 sm:grid-cols-3">
      <div><label class="field-label" for="ar-type">Type</label><select id="ar-type" name="type" class="field"><option value="threshold">threshold</option><option value="latency_spike">latency_spike</option><option value="error_rate">error_rate</option></select></div>
      <div><label class="field-label" for="ar-sev">Severity</label><select id="ar-sev" name="severity" class="field"><option value="low">low</option><option value="medium">medium</option><option value="high">high</option><option value="critical">critical</option></select></div>
      <div><label class="field-label" for="ar-threshold">Threshold</label><input id="ar-threshold" name="threshold" type="number" step="any" class="field mono" value="0" /></div>
      <div><label class="field-label" for="ar-dur">Duration (min)</label><input id="ar-dur" name="duration_minutes" type="number" class="field mono" value="10" /></div>
      <div><label class="field-label" for="ar-win">Window (min)</label><input id="ar-win" name="window_minutes" type="number" class="field mono" value="5" /></div>
    </div>
    <label class="flex items-center gap-2 text-[.74rem]"><input type="checkbox" name="enabled" checked />Enabled</label>
  `,
});

export const openNewGroupModal = createOpener({
  op: "group",
  title: "New notification group",
  subtitle: "Channels route alert events to log / webhook / etc.",
  formInner: `
    <input type="hidden" name="op" value="group" />
    <div><label class="field-label" for="ng-name">Name</label><input id="ng-name" name="name" class="field" required autofocus /></div>
    <div><label class="field-label" for="ng-channels">Channels (comma)</label><input id="ng-channels" name="channels" class="field mono" placeholder="log, webhook" /></div>
  `,
});

export const openNewWebhookModal = createOpener({
  op: "webhook",
  title: "New webhook",
  subtitle: "Subscribers receive HMAC-signed payloads.",
  formInner: `
    <input type="hidden" name="op" value="webhook" />
    <div><label class="field-label" for="wh-url">URL</label><input id="wh-url" name="url" type="url" class="field mono" required autofocus placeholder="https://example.com/hook" /></div>
    <div><label class="field-label" for="wh-events">Events (comma)</label><input id="wh-events" name="events" class="field mono" placeholder="release.activated, alert.fired" /></div>
    <div><label class="field-label" for="wh-secret">Signing secret</label><input id="wh-secret" name="secret" class="field mono" placeholder="(optional)" /></div>
  `,
});

export const openVaultKeyModal = createOpener({
  op: "vault",
  title: "Save vault key",
  subtitle: "Master key stays on the daemon; this key is round-tripped only for delete.",
  formInner: `
    <input type="hidden" name="op" value="vault" />
    <div class="grid gap-3 sm:grid-cols-2">
      <div><label class="field-label" for="vk-provider">Provider</label><input id="vk-provider" name="provider_name" class="field mono" required autofocus placeholder="openai" /></div>
      <div><label class="field-label" for="vk-name">Key name</label><input id="vk-name" name="key_name" class="field mono" required placeholder="prod-write" /></div>
    </div>
    <div><label class="field-label" for="vk-key">Key material</label><input id="vk-key" name="key" type="password" class="field mono" required placeholder="paste once, the dashboard will not display it again" /></div>
  `,
});

export const openNewUserModal = createOpener({
  op: "user",
  title: "Create user",
  subtitle: "Admin can mint a new admin key after creation.",
  formInner: `
    <input type="hidden" name="op" value="user" />
    <div class="grid gap-3 sm:grid-cols-2">
      <div><label class="field-label" for="ur-name">Name</label><input id="ur-name" name="name" class="field" required autofocus /></div>
      <div><label class="field-label" for="ur-email">Email</label><input id="ur-email" name="email" type="email" class="field" required /></div>
    </div>
    <div><label class="field-label" for="ur-role">Role</label><select id="ur-role" name="role" class="field"><option value="admin">admin</option><option value="writer">writer</option><option value="reader">reader</option></select></div>
  `,
});
