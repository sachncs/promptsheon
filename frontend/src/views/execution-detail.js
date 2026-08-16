// Execution detail page. Surfaces GET /api/v1/executions/{id} for the
// single-execution view (input, output, latency, tokens, cost).

import * as api from "../api.js";
import { escape, formatRelative, formatMoney, apiStatusLabel } from "../utils.js";
import { statusPill, pageHeader, panel, errorState, inlineBanner } from "../ui.js";

export async function renderExecutionDetail(route) {
  const root = window.document.getElementById("view");
  const id = route?.params?.id;
  if (!root) return "";
  if (!id) {
    root.innerHTML = `<div class="mt-5">${errorState({ ok: false, error: "No execution id in the URL." })}</div>`;
    return;
  }
  root.innerHTML = `<section class="panel p-5"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
  const xRes = await api.getExecution(id);
  if (!xRes.ok) {
    root.innerHTML = `<div class="mt-5">${errorState({ ...xRes, error: `Could not load execution: ${apiStatusLabel(xRes)}` })}</div>`;
    return;
  }
  const x = xRes.data;
  const hasError = Boolean(x.error);
  const breadcrumbs = `<nav class="eyebrow flex items-center gap-2" aria-label="Breadcrumb">
    <a href="#/capabilities" class="hover:text-ink">Capabilities</a>
    <span class="text-[#b2b3af]">/</span>
    <a href="#/versions/${escape(x.capability_version_id)}" class="hover:text-ink">Version</a>
    <span class="text-[#b2b3af]">/</span>
    <span>Execution</span>
  </nav>`;
  root.innerHTML = `${pageHeader({
    eyebrow: breadcrumbs,
    title: x.id,
    description: `${formatRelative(x.timestamp)} · ${x.environment || ""} · ${x.model || ""}`,
    actions: statusPill(hasError ? "Error" : "OK", hasError ? "danger" : "good"),
  })}
  <section class="mt-5 space-y-5">
    ${panel({
      eyebrow: "Execution metadata",
      title: "Telemetry",
      body: `
        <dl class="grid gap-3 sm:grid-cols-4">
          <div><dt class="eyebrow">Status</dt><dd class="mt-1">${statusPill(hasError ? "Error" : "OK", hasError ? "danger" : "good")}</dd></div>
          <div><dt class="eyebrow">Latency</dt><dd class="mt-1 text-[.78rem] font-bold mono">${escape(String(x.latency_ms || 0))}ms</dd></div>
          <div><dt class="eyebrow">Cost</dt><dd class="mt-1 text-[.78rem] font-bold mono">${escape(formatMoney(x.cost_usd || 0))}</dd></div>
          <div><dt class="eyebrow">Tokens</dt><dd class="mt-1 text-[.66rem] font-bold mono">${x.prompt_tokens || 0}p / ${x.completion_tokens || 0}c / ${x.total_tokens || 0}t</dd></div>
        </dl>
        ${hasError ? inlineBanner({ tone: "danger", message: x.error }) : ""}
      `,
    })}
    ${panel({
      eyebrow: "Inputs",
      title: "Request",
      body: `<pre class="overflow-x-auto rounded-lg bg-paper p-4 text-[.66rem] mono">${escape(JSON.stringify(x.inputs, null, 2))}</pre>`,
    })}
    ${panel({
      eyebrow: "Outputs",
      title: "Response",
      body: `<pre class="overflow-x-auto rounded-lg bg-paper p-4 text-[.66rem] mono">${escape(JSON.stringify(x.outputs, null, 2))}</pre>`,
    })}
  </section>`;
}
