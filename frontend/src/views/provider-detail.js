// Provider detail page. Surfaces GET /api/v1/providers/{name}
// and gives the operator a one-click "Test connection" form.
import * as api from "../api.js";
import { escape, formatRelative, apiStatusLabel } from "../utils.js";

export async function renderProviderDetail(route) {
  const root = window.document.getElementById("view");
  const name = route?.params?.id || route?.params?.name;
  if (!root) return "";
  if (!name) {
    root.innerHTML = `<p class="panel p-5 mt-5 text-[.78rem] text-muted">No provider name in the URL.</p>`;
    return;
  }
  root.innerHTML = `<section class="panel p-5"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
  const [pRes, listRes] = await Promise.all([
    api.getProvider(name),
    api.listProviders().catch(() => ({ ok: false }))
  ]);
  if (!pRes.ok) {
    root.innerHTML = `<p class="panel p-5 mt-5 text-[.78rem] text-muted">Could not load provider: ${escape(apiStatusLabel(pRes))}. The provider may not be registered with the running daemon.</p>`;
    return;
  }
  const p = pRes.data || {};
  const all = listRes.ok ? (listRes.data || []) : [];
  const meta = all.find((q) => q.name === name) || {};
  const config = p.config || meta.config || {};
  const supported = Array.isArray(meta.supported_models) ? meta.supported_models : (Array.isArray(p.supported_models) ? p.supported_models : []);
  root.innerHTML = render(name, p, meta, config, supported);
  attach(name, p);
}

function render(name, p, meta, config, supported) {
  return `<section class="mt-5 space-y-5">
    <article class="panel p-5 sm:p-6">
      <div class="flex items-center justify-between">
        <div>
          <div class="eyebrow flex items-center gap-2">
            <a href="#/operations/providers" class="hover:text-ink">Providers</a>
            <span class="text-[#b2b3af]">/</span>
            <span>${escape(name)}</span>
          </div>
          <h1 class="mt-2 text-[1.5rem] font-bold tracking-[-.04em]">${escape(meta.label || name)}</h1>
          <p class="mt-1 text-[.7rem] text-muted mono">${escape(name)}</p>
        </div>
        <span class="status-pill ${p.configured ? "good" : "warn"} !px-2 !py-1">${p.configured ? "configured" : "needs api key"}</span>
      </div>
      <dl class="mt-5 grid gap-3 sm:grid-cols-3">
        <div><dt class="eyebrow">Base URL</dt><dd class="mt-1 text-[.78rem] font-bold mono">${escape(config.base_url || "—")}</dd></div>
        <div><dt class="eyebrow">API key</dt><dd class="mt-1 text-[.78rem] font-bold mono">${p.configured ? escape(config.api_key_ref || "ref: (in vault)") : "not set"}</dd></div>
        <div><dt class="eyebrow">Updated</dt><dd class="mt-1 text-[.78rem] font-bold mono">${escape(formatRelative(config.updated_at))}</dd></div>
      </dl>
      <div class="mt-5 flex items-end gap-2">
        <div class="flex-1">
          <label class="eyebrow mb-2 block" for="provider-test-model">Test model</label>
          <input id="provider-test-model" class="field mono !text-[.72rem]" placeholder="e.g. gpt-4o-mini" />
        </div>
        <button id="provider-test" class="primary-button !self-end"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-refresh"/></svg>Test connection</button>
      </div>
      <pre id="provider-test-result" class="mt-3 rounded-lg bg-paper p-3 text-[.66rem] mono whitespace-pre-wrap break-all"></pre>
    </article>
    <article class="panel p-5 sm:p-6">
      <div class="eyebrow">Supported models</div>
      ${supported.length
        ? `<ul class="mt-3 grid gap-1 sm:grid-cols-3 text-[.7rem]">${supported.map((m) => `<li class="rounded-md bg-paper px-2.5 py-1 mono">${escape(m)}</li>`).join("")}</ul>`
        : `<p class="mt-3 text-[.7rem] text-muted">No supported models listed.</p>`
      }
    </article>
  </section>`;
}

function attach(name, p) {
  const root = window.document.getElementById("view");
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