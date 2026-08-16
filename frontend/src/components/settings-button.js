// Connection settings modal. Opens from the sidebar Connection
// button and the header avatar dropdown. Lets the operator paste
// a fresh API key / base URL and probe the daemon with /health.

import * as api from "../api.js";
import { loadSettings, saveSettings, clearSettings } from "../settings.js";
import { escape, apiStatusLabel } from "../utils.js";
import { openModal } from "../dialog.js";
import { inlineBanner } from "../ui.js";
import { toast } from "../toast.js";

const forms = {
  apiBase: { label: "API base URL", placeholder: "leave blank for /api via Vite proxy" },
  apiKey: { label: "API key (ps_…)", placeholder: "leave blank for auth-off dev mode" },
};

function render(current, bootstrapResult) {
  const inputs = Object.entries(forms).map(([name, field]) => `
    <div>
      <label class="field-label" for="settings-${name}">${escape(field.label)}</label>
      <input id="settings-${name}" name="${name}" class="field mono" ${name === "apiBase" ? "autofocus" : ""} value="${escape(current[name] || "")}" placeholder="${escape(field.placeholder || "")}" />
    </div>
  `).join("");
  return `${bootstrapResult ? inlineBanner({ tone: "danger", message: bootstrapResult }) : ""}
    <div class="rounded-xl border border-line bg-paper/50 p-3">
      <div class="flex items-center justify-between">
        <span class="eyebrow">Test connection</span>
        <button type="button" id="settings-test" class="quiet-button !h-7 !text-[.68rem]">Run</button>
      </div>
      <p id="settings-test-result" class="mt-2 text-[.68rem] text-muted">No connection test run yet.</p>
    </div>
    <form id="settings-form" class="space-y-4">${inputs}</form>`;
}

function footerHtml() {
  return `<button type="button" class="quiet-button" data-clear-settings>Clear</button>
    <button type="submit" form="settings-form" class="primary-button">Save and reload</button>`;
}

export async function openSettings(root, { bootstrapError = null, onSaved } = {}) {
  const current = loadSettings();
  const modal = openModal({
    title: "Promptsheon API",
    subtitle: "Override the API URL or paste a key. Stored only in this browser.",
    body: render(current, bootstrapError),
    footer: footerHtml(),
    size: "narrow",
  });

  const form = modal.root.querySelector("#settings-form");
  form?.addEventListener("submit", (event) => {
    event.preventDefault();
    const data = new FormData(form);
    saveSettings({
      apiBase: (data.get("apiBase") || "").toString().trim(),
      apiKey: (data.get("apiKey") || "").toString().trim(),
    });
    toast.success("Settings saved", "Reloading…");
    setTimeout(() => window.location.reload(), 250);
  });
  modal.root.querySelector("[data-clear-settings]")?.addEventListener("click", () => {
    clearSettings();
    toast.success("Settings cleared", "Reloading…");
    setTimeout(() => window.location.reload(), 250);
  });
  const testButton = modal.root.querySelector("#settings-test");
  testButton?.addEventListener("click", async () => {
    const out = modal.root.querySelector("#settings-test-result");
    out.textContent = "Testing…";
    out.className = "mt-2 text-[.68rem] text-muted";
    const probe = await api.apiFetch("/health", { retry: false, timeout: 4000 });
    if (probe.ok) {
      out.textContent = `Connected to daemon (v${escape(probe.data?.version || "?")}, uptime ${escape(probe.data?.uptime || "?")}).`;
      out.className = "mt-2 rounded-md bg-lime/15 px-2 py-1.5 text-[.68rem] text-[#3b641b]";
      return;
    }
    const fallback = await fetch("/health").then((r) => r.ok ? r.json() : null).catch(() => null);
    if (fallback) {
      out.textContent = `Direct connection failed (${apiStatusLabel(probe)}); Vite proxy works (v${escape(fallback.version || "?")}).`;
      out.className = "mt-2 rounded-md bg-amber-100 px-2 py-1.5 text-[.68rem] text-amber-800";
      return;
    }
    out.textContent = `${apiStatusLabel(probe)} — is the daemon running on ${escape(current.apiBase || "/health")}?`;
    out.className = "mt-2 rounded-md bg-rose-100 px-2 py-1.5 text-[.68rem] text-rose-800";
  });
}
