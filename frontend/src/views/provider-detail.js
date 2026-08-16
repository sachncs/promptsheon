// Provider detail page. Surfaces GET /api/v1/providers/{name} and
// gives the operator a one-click "Test connection" form.

import * as api from "../api.js";
import { escape, formatRelative, apiStatusLabel } from "../utils.js";
import { statusPill, pageHeader, panel, errorState } from "../ui.js";

export async function renderProviderDetail(route) {
  const root = window.document.getElementById("view");
  const name = route?.params?.id || route?.params?.name;
  if (!root) return "";
  if (!name) {
    root.innerHTML = `<div class="mt-5">${errorState({ ok: false, error: "No provider name in the URL." })}</div>`;
    return;
  }
  root.innerHTML = `<section class="panel p-5"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
  const [pRes, listRes] = await Promise.all([
    api.getProvider(name),
    api.listProviders().catch(() => ({ ok: false })),
  ]);
  if (!pRes.ok) {
    root.innerHTML = `<div class="mt-5">${errorState({ ...pRes, error: `Could not load provider: ${apiStatusLabel(pRes)}. The provider may not be registered with the running daemon.` })}</div>`;
    return;
  }
  const p = pRes.data || {};
  const config = p.config || {};
  const supported = Array.isArray(p.supported_models) ? p.supported_models : [];
  const breadcrumbs = `<nav class="eyebrow flex items-center gap-2" aria-label="Breadcrumb">
    <a href="#/operations/providers" class="hover:text-ink">Providers</a>
    <span class="text-[#b2b3af]">/</span>
    <span>${escape(name)}</span>
  </nav>`;

  root.innerHTML = `${pageHeader({
    eyebrow: breadcrumbs,
    title: name,
    description: name,
    actions: statusPill(p.configured ? "configured" : "needs api key", p.configured ? "good" : "warn"),
  })}
  <section class="mt-5 space-y-5">
    ${panel({
      eyebrow: "Provider metadata",
      title: "Configuration",
      body: `
        <dl class="grid gap-3 sm:grid-cols-3">
          <div><dt class="eyebrow">Base URL</dt><dd class="mt-1 text-[.78rem] font-bold mono">${escape(config.base_url || "—")}</dd></div>
          <div><dt class="eyebrow">API key</dt><dd class="mt-1 text-[.78rem] font-bold mono">${p.configured ? escape(config.api_key_ref || "ref: (in vault)") : "not set"}</dd></div>
          <div><dt class="eyebrow">Updated</dt><dd class="mt-1 text-[.78rem] font-bold mono">${escape(formatRelative(config.updated_at))}</dd></div>
        </dl>
        <div class="mt-5 flex items-end gap-2">
          <div class="flex-1">
            <label class="field-label" for="provider-test-model">Test model</label>
            <input id="provider-test-model" class="field mono !text-[.72rem]" placeholder="e.g. gpt-4o-mini" />
          </div>
          <button id="provider-test" class="primary-button !self-end"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-refresh"/></svg>Test connection</button>
        </div>
        <pre id="provider-test-result" class="mt-3 rounded-lg bg-paper p-3 text-[.66rem] mono whitespace-pre-wrap break-all"></pre>
      `,
    })}
    ${panel({
      eyebrow: "Supported models",
      title: "Catalog",
      body: supported.length
        ? `<ul class="mt-3 grid gap-1 sm:grid-cols-3 text-[.7rem]">${supported.map((m) => `<li class="rounded-md bg-paper px-2.5 py-1 mono">${escape(m)}</li>`).join("")}</ul>`
        : `<p class="mt-3 text-[.7rem] text-muted">No supported models listed.</p>`,
    })}
  </section>`;

  root.querySelector("#provider-test")?.addEventListener("click", async () => {
    const model = root.querySelector("#provider-test-model")?.value?.trim() || "";
    const slot = root.querySelector("#provider-test-result");
    slot.textContent = `Testing ${name}…`;
    const result = await api.testProvider(name, model);
    if (!result.ok) {
      slot.textContent = `${name} test failed: ${apiStatusLabel(result)}`;
      return;
    }
    const data = result.data || {};
    const latency = data.latency_ms != null ? `${data.latency_ms}ms` : "?ms";
    const output = typeof data.content === "string" ? ` — "${data.content.replace(/[\n\r]/g, " ").slice(0, 80)}…"` : "";
    slot.textContent = `${name}: ${data.status || "ok"} (${latency})${output}`;
  });
}
