// Settings view. Surfaces the four /api/v1/settings routes:
// list, get, set, delete. Used as the "Settings" tab in the
// operations area; the value column is the JSON-encoded value
// (masked to "***" by the backend for secret-shaped keys).
import * as api from "../api.js";
import { escape, formatRelative, apiStatusLabel } from "../utils.js";

const SETTINGS_TABS = [
  { key: "settings", label: "Settings", href: "#/operations/settings" }
];

export function settingsTab() {
  return SETTINGS_TABS[0].label;
}

export function isSettingsTab(tab) {
  return tab === "settings";
}

export async function renderSettingsTab(root) {
  if (!root) return "";
  root.innerHTML = `<section class="panel p-5"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
  const res = await api.listSettings();
  if (!res.ok) {
    root.innerHTML = `<p class="panel p-5 mt-5 text-[.78rem] text-muted">Settings unavailable${res.error ? ` (${escape(res.error)})` : ""}. The daemon's settings layer is unwired (this is the no-auth default).</p>`;
    return;
  }
  // Backend returns { items: [...] }; tolerate a bare array too.
  const items = Array.isArray(res.data) ? res.data : (res.data?.items || []);
  root.innerHTML = `<section class="mt-5 space-y-5">
    <article class="panel p-5 sm:p-6">
      <div class="flex items-center justify-between">
        <div>
          <div class="eyebrow">System settings</div>
          <h2 class="mt-1 text-[1rem] font-bold">${items.length} configured</h2>
        </div>
        <button id="settings-new" class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg> New key</button>
      </div>
      <p class="mt-2 text-[.66rem] text-muted">Writes require the <code>settings:write</code> permission (admin by default). The daemon may be in <code>env-only</code> mode; in that case the writes return 403 regardless of the caller's role.</p>
      <form id="settings-form" class="hidden mt-4 space-y-3 rounded-xl bg-paper p-4">
        <div class="grid gap-3 sm:grid-cols-[180px_1fr]">
          <div><label class="eyebrow mb-2 block" for="set-key">Key</label><input id="set-key" name="key" class="field mono" required placeholder="e.g. feature.beta_pricing" /></div>
          <div><label class="eyebrow mb-2 block" for="set-value">Value (JSON-encoded)</label><input id="set-value" name="value" class="field mono" required placeholder='e.g. {"enabled": true}' /></div>
        </div>
        <p id="set-error" class="hidden rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800"></p>
        <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
          <button type="button" class="quiet-button" data-collapse-settings-form>Cancel</button>
          <button type="submit" class="primary-button">Save</button>
        </div>
      </form>
      <div class="mt-4">${renderList(items)}</div>
    </article>
  </section>`;
  attach(root);
}

function renderList(items) {
  if (!items.length) {
    return `<p class="mt-3 text-[.7rem] text-muted">No settings yet. Create one to override the daemon's defaults at runtime.</p>`;
  }
  return `<table class="mt-2 w-full text-[.7rem]"><thead><tr class="text-left text-[.6rem] uppercase tracking-wider text-muted"><th class="py-1 font-bold">Key</th><th class="py-1 font-bold">Value</th><th class="py-1 font-bold">Updated</th><th class="py-1 font-bold text-right"></th></tr></thead><tbody>${items.map((it) => `<tr class="border-t border-line/60" data-setting-row="${escape(it.key || it.id || "")}">
    <td class="py-2 mono text-[.68rem]">${escape(it.key || it.id || "—")}</td>
    <td class="py-2 mono text-[.66rem]">${escape(formatValue(it.value))}</td>
    <td class="py-2 mono">${escape(formatRelative(it.updated_at || it.created_at))}</td>
    <td class="py-2 text-right"><button data-setting-delete="${escape(it.key || it.id || "")}" data-setting-name="${escape(it.key || it.id || "")}" class="rounded-md bg-rose-50 px-2 py-1 text-[.62rem] font-bold text-rose-700 hover:bg-rose-100">Delete</button></td>
  </tr>`).join("")}</tbody></table>`;
}

function formatValue(v) {
  if (v == null) return "—";
  if (typeof v === "string") return v;
  try { return JSON.stringify(v); } catch { return String(v); }
}

function attach(root) {
  root.querySelector("#settings-new")?.addEventListener("click", () => {
    const form = root.querySelector("#settings-form");
    form?.classList.toggle("hidden");
    form?.querySelector("input,textarea")?.focus();
  });
  root.querySelector("[data-collapse-settings-form]")?.addEventListener("click", () => {
    const form = root.querySelector("#settings-form");
    form?.classList.add("hidden");
    const err = root.querySelector("#set-error");
    err?.classList.add("hidden");
  });
  root.querySelector("#settings-form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const data = new FormData(event.target);
    const key = String(data.get("key") || "").trim();
    const valueRaw = String(data.get("value") || "").trim();
    const errSlot = root.querySelector("#set-error");
    errSlot.classList.add("hidden");
    if (!key || !valueRaw) {
      errSlot.textContent = "Key and value are required.";
      errSlot.classList.remove("hidden");
      return;
    }
    // The value column accepts JSON; pass it through verbatim
    // so a non-JSON string survives as a string-typed setting.
    let value = valueRaw;
    try { value = JSON.parse(valueRaw); } catch { /* keep as string */ }
    const result = await api.setSetting(key, value);
    if (!result.ok) {
      errSlot.textContent = `Save failed: ${apiStatusLabel(result)}`;
      errSlot.classList.remove("hidden");
      return;
    }
    event.target.reset();
    const refreshed = await api.listSettings();
    if (refreshed.ok) {
      const list = root.querySelector("[data-setting-row]")?.parentElement?.parentElement;
      if (list) list.outerHTML = renderList(refreshed.data || []);
      attach(root);
    }
  });
  root.querySelectorAll("[data-setting-delete]").forEach((b) =>
    b.addEventListener("click", async () => {
      const key = b.dataset.settingDelete;
      if (!key) return;
      if (!window.confirm(`Delete setting "${key}"?`)) return;
      const result = await api.deleteSetting(key);
      if (!result.ok) {
        window.alert(`Delete failed: ${apiStatusLabel(result)}`);
        return;
      }
      const refreshed = await api.listSettings();
      if (refreshed.ok) {
        const list = b.closest("table");
        if (list) list.outerHTML = renderList(refreshed.data || []);
        attach(root);
      }
    })
  );
}