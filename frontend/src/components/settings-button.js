import * as api from "../api.js";
import { loadSettings, saveSettings, clearSettings } from "../settings.js";
import { escape, apiStatusLabel } from "../utils.js";

const forms = {
  apiBase: { label: "API base URL", type: "text", placeholder: "leave blank for /api via Vite proxy" },
  apiKey: { label: "API key (ps_…)", type: "text", placeholder: "leave blank for auth-off dev mode" }
};

function render(current, bootstrapResult) {
  return `
    <div class="modal-backdrop" role="presentation">
      <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="settings-title">
        <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
          <div>
            <div class="eyebrow">Connection</div>
            <h2 id="settings-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">Promptsheon API</h2>
            <p class="mt-1 text-[.7rem] text-muted">Override the API URL or paste a key. Stored only in this browser.</p>
          </div>
          <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
        </div>
        <form id="settings-form" class="space-y-4 px-5 py-5 sm:px-6">
          ${Object.entries(forms).map(([name, field]) => `
            <div>
              <label class="eyebrow mb-2 block" for="settings-${name}">${escape(field.label)}</label>
              <input id="settings-${name}" name="${name}" type="${field.type}" class="field mono" ${name === "apiBase" ? "data-autofocus" : ""} value="${escape(current[name] || "")}" placeholder="${escape(field.placeholder || "")}" />
            </div>
          `).join("")}
          <div class="rounded-xl border border-line bg-paper/50 p-3">
            <div class="flex items-center justify-between">
              <span class="eyebrow">Test connection</span>
              <button type="button" id="settings-test" class="quiet-button !h-7 !text-[.68rem]">Run</button>
            </div>
            <p id="settings-test-result" class="mt-2 text-[.68rem] text-muted">No connection test run yet.</p>
          </div>
          ${bootstrapResult ? `<p class="rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800">${escape(bootstrapResult)}</p>` : ""}
          <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
            <button type="button" class="quiet-button" data-clear-settings>Clear</button>
            <button type="submit" class="primary-button">Save and reload</button>
          </div>
        </form>
      </section>
    </div>
  `;
}

function attach(root, current, { onSaved, onCleared } = {}) {
  const form = root.querySelector("#settings-form");
  form.addEventListener("submit", (event) => {
    event.preventDefault();
    const data = new FormData(form);
    saveSettings({
      apiBase: (data.get("apiBase") || "").toString().trim(),
      apiKey: (data.get("apiKey") || "").toString().trim()
    });
    window.location.reload();
  });
  form.querySelector("[data-clear-settings]")?.addEventListener("click", () => {
    clearSettings();
    window.location.reload();
  });
  const testButton = root.querySelector("#settings-test");
  testButton?.addEventListener("click", async () => {
    const out = root.querySelector("#settings-test-result");
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

export async function openSettings(root, { bootstrapError = null, onSaved } = {}) {
  const current = loadSettings();
  root.innerHTML = render(current, bootstrapError);
  attach(root, current, { onSaved });
}
