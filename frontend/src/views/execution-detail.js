// Execution detail page. Surfaces GET /api/v1/executions/{id}
// for the single-execution view (input, output, latency,
// tokens, cost).
import * as api from "../api.js";
import { escape, formatRelative, formatMoney, apiStatusLabel } from "../utils.js";

function pill(text, tone = "neutral") {
  return `<span class="status-pill ${tone} !px-2 !py-1"><span class="status-dot"></span>${escape(text)}</span>`;
}

export async function renderExecutionDetail(route) {
  const root = window.document.getElementById("view");
  const id = route?.params?.id;
  if (!root) return "";
  if (!id) {
    root.innerHTML = `<p class="panel p-5 mt-5 text-[.78rem] text-muted">No execution id in the URL.</p>`;
    return;
  }
  root.innerHTML = `<section class="panel p-5"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
  const xRes = await api.getExecution(id);
  if (!xRes.ok) {
    root.innerHTML = `<p class="panel p-5 mt-5 text-[.78rem] text-muted">Could not load execution: ${escape(apiStatusLabel(xRes))}</p>`;
    return;
  }
  const x = xRes.data;
  const hasError = Boolean(x.error);
  root.innerHTML = `<section class="mt-5 space-y-5">
    <article class="panel p-5 sm:p-6">
      <div class="eyebrow flex items-center gap-2">
        <a href="#/capabilities" class="hover:text-ink">Capabilities</a>
        <span class="text-[#b2b3af]">/</span>
        <a href="#/versions/${escape(x.capability_version_id)}" class="hover:text-ink">Version</a>
        <span class="text-[#b2b3af]">/</span>
        <span>Execution</span>
      </div>
      <h1 class="mt-2 text-[1.3rem] font-bold tracking-[-.04em] mono">${escape(x.id)}</h1>
      <p class="mt-1 text-[.7rem] text-muted">${escape(formatRelative(x.timestamp))} · ${escape(x.environment || "")} · ${escape(x.model || "")}</p>
      <dl class="mt-5 grid gap-3 sm:grid-cols-4">
        <div><dt class="eyebrow">Status</dt><dd class="mt-1">${pill(hasError ? "Error" : "OK", hasError ? "danger" : "good")}</dd></div>
        <div><dt class="eyebrow">Latency</dt><dd class="mt-1 text-[.78rem] font-bold mono">${escape(String(x.latency_ms || 0))}ms</dd></div>
        <div><dt class="eyebrow">Cost</dt><dd class="mt-1 text-[.78rem] font-bold mono">${escape(formatMoney(x.cost_usd || 0))}</dd></div>
        <div><dt class="eyebrow">Tokens</dt><dd class="mt-1 text-[.66rem] font-bold mono">${x.prompt_tokens || 0}p / ${x.completion_tokens || 0}c / ${x.total_tokens || 0}t</dd></div>
      </dl>
      ${hasError ? `<p class="mt-4 rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800 mono">${escape(x.error)}</p>` : ""}
    </article>
    <article class="panel p-5 sm:p-6">
      <div class="eyebrow">Inputs</div>
      <pre class="mt-3 overflow-x-auto rounded-lg bg-paper p-4 text-[.66rem] mono">${escape(JSON.stringify(x.inputs, null, 2))}</pre>
    </article>
    <article class="panel p-5 sm:p-6">
      <div class="eyebrow">Outputs</div>
      <pre class="mt-3 overflow-x-auto rounded-lg bg-paper p-4 text-[.66rem] mono">${escape(JSON.stringify(x.outputs, null, 2))}</pre>
    </article>
  </section>`;
}