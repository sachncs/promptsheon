// Settings view. Surfaces the four /api/v1/settings routes: list,
// get, set, delete. Used as the "Settings" tab in the operations
// area; the value column is the JSON-encoded value (masked to "***"
// by the server for secret-shaped keys).
//
// Migrated to ui.js primitives — pageHeader / panel / dataTable /
// emptyState / errorState / inlineBanner — so the operations tabs
// share a single rendering contract.

import * as api from "../api.js";
import { escape, formatRelative, apiStatusLabel } from "../utils.js";
import { pageHeader, panel, dataTable, emptyState, errorState, inlineBanner } from "../ui.js";

function formatValue(v) {
  if (v == null) return "—";
  if (typeof v === "string") return v;
  try { return JSON.stringify(v); } catch { return String(v); }
}

function renderList(items) {
  if (!items.length) {
    return emptyState("No settings yet. Create one to override the daemon's defaults at runtime.", { icon: "icon-settings" });
  }
  return dataTable({
    columns: [
      { key: "key", label: "Key", render: (it) => `<span class="mono">${escape(it.key || it.id || "—")}</span>` },
      { key: "value", label: "Value", render: (it) => `<span class="mono text-muted truncate max-w-[24rem] inline-block align-middle">${escape(formatValue(it.value))}</span>` },
      { key: "updated", label: "Updated", render: (it) => `<span class="mono text-muted">${escape(formatRelative(it.updated_at || it.created_at))}</span>` },
      { key: "actions", label: "", align: "right", render: (it) => `<button data-setting-delete="${escape(it.key || it.id || "")}" data-setting-name="${escape(it.key || it.id || "")}" class="danger-button !text-[.6rem]">Delete</button>` },
    ],
    rows: items,
    emptyMessage: "No settings yet.",
    emptyIcon: "icon-settings",
  });
}

export async function renderSettingsTab(root) {
  if (!root) return "";
  root.innerHTML = `<section class="panel p-5"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
  const res = await api.listSettings();
  if (!res.ok) {
    root.innerHTML = `<div class="mt-5">${errorState({ ...res, error: `Settings unavailable${res.error ? ` (${res.error})` : ""}. The daemon's settings layer is unwired (this is the no-auth default).` })}</div>`;
    return;
  }
  const items = Array.isArray(res.data) ? res.data : (res.data?.items || []);

  root.innerHTML = `${pageHeader({
    eyebrow: "Operations",
    title: "Settings",
    description: `Writes require the <code>settings:write</code> permission (admin by default). The daemon may be in <code>env-only</code> mode; in that case the writes return 403 regardless of the caller's role.`,
    actions: `<button id="settings-new" class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg> New key</button>`,
  })}
  <section class="mt-5 space-y-5">
    ${panel({
      eyebrow: "System settings",
      title: `${items.length} configured`,
      body: `
        <form id="settings-form" class="hidden mt-4 space-y-3 rounded-xl bg-paper p-4">
          <div class="grid gap-3 sm:grid-cols-[180px_1fr]">
            <div><label class="field-label" for="set-key">Key</label><input id="set-key" name="key" class="field mono" required placeholder="e.g. feature.beta_pricing" /></div>
            <div><label class="field-label" for="set-value">Value (JSON-encoded)</label><input id="set-value" name="value" class="field mono" required placeholder='{"enabled": true}' /></div>
          </div>
          <p id="set-error" class="hidden"></p>
          <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
            <button type="button" class="quiet-button" data-collapse-settings-form>Cancel</button>
            <button type="submit" class="primary-button">Save</button>
          </div>
        </form>
        <div class="mt-4">${renderList(items)}</div>
      `,
    })}
  </section>`;
  attach(root);
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
    if (err) {
      err.classList.add("hidden");
      err.innerHTML = "";
    }
  });
  root.querySelector("#settings-form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const data = new FormData(event.target);
    const key = String(data.get("key") || "").trim();
    const valueRaw = String(data.get("value") || "").trim();
    const errSlot = root.querySelector("#set-error");
    errSlot.classList.add("hidden");
    errSlot.innerHTML = "";
    if (!key || !valueRaw) {
      errSlot.innerHTML = inlineBanner({ tone: "danger", message: "Key and value are required." });
      errSlot.classList.remove("hidden");
      return;
    }
    let value = valueRaw;
    try { value = JSON.parse(valueRaw); } catch { /* keep as string */ }
    const result = await api.setSetting(key, value);
    if (!result.ok) {
      errSlot.innerHTML = inlineBanner({ tone: "danger", message: `Save failed: ${apiStatusLabel(result)}` });
      errSlot.classList.remove("hidden");
      return;
    }
    event.target.reset();
    const refreshed = await api.listSettings();
    if (refreshed.ok) {
      const list = root.querySelector("[data-setting-row]")?.closest("table");
      if (list) list.outerHTML = renderList(Array.isArray(refreshed.data) ? refreshed.data : (refreshed.data?.items || []));
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
        if (list) list.outerHTML = renderList(Array.isArray(refreshed.data) ? refreshed.data : (refreshed.data?.items || []));
        attach(root);
      }
    }),
  );
}
