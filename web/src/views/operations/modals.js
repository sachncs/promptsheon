import * as api from "../../api.js";
import { escape, apiStatusLabel } from "../../utils.js";

function renderShell(title, intro, body) {
  return `<div class="modal-backdrop" role="presentation">
    <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="op-modal-title">
      <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
        <div><div class="eyebrow">Operations</div><h2 id="op-modal-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">${escape(title)}</h2>${intro ? `<p class="mt-1 text-[.7rem] text-muted">${escape(intro)}</p>` : ""}</div>
        <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
      </div>
      <form class="space-y-4 px-5 py-5 sm:px-6">
        ${body}
        <p class="op-modal-error hidden rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800"></p>
        <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
          <button type="button" class="quiet-button" data-close-modal>Cancel</button>
          <button type="submit" class="primary-button">Save</button>
        </div>
      </form>
    </section>
  </div>`;
}

function bind(root, refresh) {
  root.querySelector("[data-close-modal]")?.addEventListener("click", () => root.replaceChildren());
  root.querySelector("form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const data = new FormData(event.target);
    const op = event.target.closest("form").dataset.op;
    const error = root.querySelector(".op-modal-error");
    error.classList.add("hidden");
    const submit = event.target.querySelector("button[type=submit]");
    submit.disabled = true;
    let payload = null;
    let result = null;
    try {
      if (op === "rule") {
        payload = {
          name: (data.get("name") || "").toString().trim(),
          type: (data.get("type") || "threshold").toString(),
          severity: (data.get("severity") || "medium").toString(),
          threshold: Number(data.get("threshold") || 0),
          duration_minutes: Number(data.get("duration_minutes") || 0),
          window_minutes: Number(data.get("window_minutes") || 0),
          enabled: data.get("enabled") === "on"
        };
        result = await api.createAlertRule(payload);
      } else if (op === "group") {
        payload = {
          name: (data.get("name") || "").toString().trim(),
          channels: (data.get("channels") || "log").toString().split(",").map((s) => s.trim()).filter(Boolean)
        };
        result = await api.createNotificationGroup(payload);
      } else if (op === "webhook") {
        payload = {
          url: (data.get("url") || "").toString().trim(),
          events: (data.get("events") || "").toString().split(",").map((s) => s.trim()).filter(Boolean),
          secret: (data.get("secret") || "").toString()
        };
        result = await api.createWebhook(payload);
      } else if (op === "vault") {
        payload = {
          provider_name: (data.get("provider_name") || "").toString().trim(),
          key_name: (data.get("key_name") || "").toString().trim(),
          key: (data.get("key") || "").toString()
        };
        result = await api.saveVaultKey(payload);
      } else if (op === "user") {
        payload = {
          email: (data.get("email") || "").toString().trim(),
          name: (data.get("name") || "").toString().trim(),
          role: (data.get("role") || "reader").toString()
        };
        result = await api.createUser(payload);
      }
    } catch (e) {
      error.textContent = String(e?.message || e);
      error.classList.remove("hidden");
      submit.disabled = false;
      return;
    }
    submit.disabled = false;
    if (!result || !result.ok) {
      error.textContent = apiStatusLabel(result || { error: "no response" });
      error.classList.remove("hidden");
      return;
    }
    root.replaceChildren();
    if (typeof refresh === "function") refresh();
  });
}

export async function openNewRuleModal(root, refresh) {
  if (!root) return;
  const body = `
    <input type="hidden" name="op" value="rule" />
    <div><label class="eyebrow mb-2 block" for="ar-name">Name</label><input id="ar-name" name="name" class="field" required data-autofocus /></div>
    <div class="grid gap-3 sm:grid-cols-3">
      <div><label class="eyebrow mb-2 block" for="ar-type">Type</label><select id="ar-type" name="type" class="field"><option value="threshold">threshold</option><option value="latency_spike">latency_spike</option><option value="error_rate">error_rate</option></select></div>
      <div><label class="eyebrow mb-2 block" for="ar-sev">Severity</label><select id="ar-sev" name="severity" class="field"><option value="low">low</option><option value="medium">medium</option><option value="high">high</option><option value="critical">critical</option></select></div>
      <div><label class="eyebrow mb-2 block" for="ar-threshold">Threshold</label><input id="ar-threshold" name="threshold" type="number" step="any" class="field mono" value="0" /></div>
      <div><label class="eyebrow mb-2 block" for="ar-dur">Duration (min)</label><input id="ar-dur" name="duration_minutes" type="number" class="field mono" value="10" /></div>
      <div><label class="eyebrow mb-2 block" for="ar-win">Window (min)</label><input id="ar-win" name="window_minutes" type="number" class="field mono" value="5" /></div>
    </div>
    <label class="flex items-center gap-2 text-[.74rem]"><input type="checkbox" name="enabled" checked />Enabled</label>`;
  root.innerHTML = renderShell("New alert rule", "Threshold + duration + window define when the rule fires.", body);
  bind(root, refresh);
}

export async function openNewGroupModal(root, refresh) {
  if (!root) return;
  const body = `
    <input type="hidden" name="op" value="group" />
    <div><label class="eyebrow mb-2 block" for="ng-name">Name</label><input id="ng-name" name="name" class="field" required data-autofocus /></div>
    <div><label class="eyebrow mb-2 block" for="ng-channels">Channels (comma)</label><input id="ng-channels" name="channels" class="field mono" placeholder="log, webhook" /></div>`;
  root.innerHTML = renderShell("New notification group", "Channels route alert events to log / webhook / etc.", body);
  bind(root, refresh);
}

export async function openNewWebhookModal(root, refresh) {
  if (!root) return;
  const body = `
    <input type="hidden" name="op" value="webhook" />
    <div><label class="eyebrow mb-2 block" for="wh-url">URL</label><input id="wh-url" name="url" type="url" class="field mono" required data-autofocus placeholder="https://example.com/hook" /></div>
    <div><label class="eyebrow mb-2 block" for="wh-events">Events (comma)</label><input id="wh-events" name="events" class="field mono" placeholder="release.activated, alert.fired" /></div>
    <div><label class="eyebrow mb-2 block" for="wh-secret">Signing secret</label><input id="wh-secret" name="secret" class="field mono" placeholder="(optional)" /></div>`;
  root.innerHTML = renderShell("New webhook", "Subscribers receive HMAC-signed payloads.", body);
  bind(root, refresh);
}

export async function openVaultKeyModal(root, refresh) {
  if (!root) return;
  const body = `
    <input type="hidden" name="op" value="vault" />
    <div class="grid gap-3 sm:grid-cols-2">
      <div><label class="eyebrow mb-2 block" for="vk-provider">Provider</label><input id="vk-provider" name="provider_name" class="field mono" required data-autofocus placeholder="openai" /></div>
      <div><label class="eyebrow mb-2 block" for="vk-name">Key name</label><input id="vk-name" name="key_name" class="field mono" required placeholder="prod-write" /></div>
    </div>
    <div><label class="eyebrow mb-2 block" for="vk-key">Key material</label><input id="vk-key" name="key" type="password" class="field mono" required placeholder="paste once, the dashboard will not display it again" /></div>`;
  root.innerHTML = renderShell("Save vault key", "Master key stays on the daemon; this key is round-tripped only for delete.", body);
  bind(root, refresh);
}

export async function openNewUserModal(root, refresh) {
  if (!root) return;
  const body = `
    <input type="hidden" name="op" value="user" />
    <div class="grid gap-3 sm:grid-cols-2">
      <div><label class="eyebrow mb-2 block" for="ur-name">Name</label><input id="ur-name" name="name" class="field" required data-autofocus /></div>
      <div><label class="eyebrow mb-2 block" for="ur-email">Email</label><input id="ur-email" name="email" type="email" class="field" required /></div>
    </div>
    <div><label class="eyebrow mb-2 block" for="ur-role">Role</label><select id="ur-role" name="role" class="field"><option value="admin">admin</option><option value="writer">writer</option><option value="reader">reader</option></select></div>`;
  root.innerHTML = renderShell("Create user", "Admin can mint a new admin key after creation.", body);
  bind(root, refresh);
}
