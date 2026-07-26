import * as api from "../api.js";
import { escape, formatCompact, formatInteger, formatMoney, formatPercent, formatRelative, apiStatusLabel } from "../utils.js";

function pill(text, tone = "neutral") {
  return `<span class="status-pill ${tone} !px-2 !py-1"><span class="status-dot"></span>${escape(text)}</span>`;
}

function card(eyebrow, value, sub, tone = "neutral") {
  return `<article class="panel p-5"><div class="flex items-start justify-between"><span class="eyebrow">${escape(eyebrow)}</span></div><div class="mt-6 flex items-end justify-between gap-3"><span class="metric-value">${escape(value)}</span><span class="status-pill ${tone}"><span class="status-dot"></span>${escape(sub)}</span></div></article>`;
}

function metricCards(summary) {
  if (!summary || !summary.ok || !summary.data) {
    const msg = summary?.status === 401 ? "Live counters unlock once an API key is set." : summary?.error ? `Unavailable (${escape(summary.error)})` : "Loading live counters…";
    return `<div class="panel p-5 text-[.72rem] text-muted">${msg}</div>`;
  }
  const api = summary.data.api_metrics || {};
  const llm = summary.data.llm_metrics || {};
  const review = summary.data.review_metrics || {};
  const guardrail = summary.data.guardrail_metrics || {};
  const evalMetrics = summary.data.eval_metrics || {};
  const workflow = summary.data.workflow_metrics || {};
  const errorRate = api.error_rate || 0;
  return `<div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
    ${card("API requests", formatCompact(api.total_requests), errorRate > 0 ? "Attention" : "Healthy", errorRate > 0 ? "warn" : "good")}
    ${card("LLM spend", formatMoney(llm.total_cost_usd), `${formatCompact(llm.total_calls)} calls`, "neutral")}
    ${card("Reviews", formatInteger(review.total_reviews), `${formatInteger(review.pending_count)} pending`, review.pending_count > 0 ? "warn" : "good")}
    ${card("Guardrails", formatInteger((guardrail.passes || 0) + (guardrail.blocks || 0)), `${formatInteger(guardrail.blocks || 0)} blocks`, guardrail.blocks > 0 ? "warn" : "good")}
    ${card("Eval runs", formatInteger(evalMetrics.total_runs || 0), `${formatInteger(evalMetrics.total_cases || 0)} cases`, "neutral")}
    ${card("Workflows", formatInteger(workflow.total_runs || 0), `${formatInteger(workflow.active_count || 0)} active`, workflow.active_count > 0 ? "good" : "neutral")}
    ${card("Bandit", formatInteger(summary.data.bandit_metrics?.selections_total || 0), `run ${escape(summary.data.bandit_metrics?.current_run_id || "—")}`, "neutral")}
    ${card("Hallucination", typeof summary.data.hallucination_metrics?.avg_score === "number" ? summary.data.hallucination_metrics.avg_score.toFixed(2) : "—", `p95 ${typeof summary.data.hallucination_metrics?.p95_score === "number" ? summary.data.hallucination_metrics.p95_score.toFixed(2) : "—"}`, "neutral")}
  </div>`;
}

function latencyRow(label, ms, total) {
  const pct = total > 0 ? (ms / total) * 100 : 0;
  return `<div><div class="flex items-center justify-between"><span class="text-[.7rem] font-semibold">${escape(label)}</span><span class="mono text-[.66rem] text-muted">${escape(ms.toFixed(2))}ms</span></div><div class="mt-1 h-2 overflow-hidden rounded-full bg-paper"><div class="h-full rounded-full bg-[#789c35]" style="width:${Math.min(100, pct).toFixed(1)}%"></div></div></div>`;
}

function latencyBreakdown(summary) {
  if (!summary || !summary.ok || !summary.data) {
    return `<p class="text-[.66rem] text-muted">Latency breakdown unavailable.</p>`;
  }
  const api = summary.data.api_metrics || {};
  const total = (api.p50_latency_ms || 0) + (api.p95_latency_ms || 0) + (api.p99_latency_ms || 0);
  return `<div class="space-y-3">${latencyRow("p50", api.p50_latency_ms || 0, total)}${latencyRow("p95", api.p95_latency_ms || 0, total)}${latencyRow("p99", api.p99_latency_ms || 0, total)}</div>`;
}

function topCapabilitiesChart(topCaps) {
  if (!topCaps || !topCaps.ok || !Array.isArray(topCaps.data?.capabilities) || !topCaps.data.capabilities.length) {
    const tone = !topCaps || topCaps.ok === false ? (topCaps?.status === 429 ? "warn" : "neutral") : "neutral";
    const text = topCaps?.status === 429 ? "Top capabilities feed rate-limited. Retrying." : !topCaps ? "Loading…" : "No recorded capability calls yet.";
    return `<p class="text-[.66rem] text-muted">${escape(text)}</p>`;
  }
  const rows = topCaps.data.capabilities.slice(0, 10);
  const maxCount = Math.max(...rows.map((r) => r.count || 0), 1);
  return `<div class="space-y-3">${rows.map((cap) => {
    const count = cap.count || 0;
    const width = (count / maxCount) * 100;
    return `<div><div class="flex items-center justify-between"><a href="#/capabilities/${escape(cap.id || "")}" class="text-[.7rem] font-bold text-ink truncate max-w-[60%]">${escape(cap.name || cap.id || "Unknown")}</a><span class="mono text-[.66rem] text-muted">${formatCompact(count)} calls</span></div><div class="mt-1 h-2 overflow-hidden rounded-full bg-paper"><div class="h-full rounded-full bg-[#6878ff]" style="width:${width.toFixed(1)}%"></div></div>${typeof cap.avg_latency_ms === "number" ? `<div class="mt-0.5 flex justify-between text-[.58rem] text-muted"><span>${formatRelative(cap.last_used)}</span><span>avg ${escape(Math.round(cap.avg_latency_ms))}ms</span></div>` : ""}</div>`;
  }).join("")}</div>`;
}

export async function renderObservability(route) {
  const root = window.document.getElementById("view");
  if (!root) return "";
  root.innerHTML = `<section class="panel p-6"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
  const [metrics, topCaps] = await api.sequential([
    () => api.getMetricsSummary(),
    () => api.getTopCapabilities()
  ], { delayMs: 60 });

  const shell = `
    <section class="flex flex-wrap items-end justify-between gap-3">
      <div>
        <div class="eyebrow">Operations</div>
        <h1 class="mt-2 text-[1.4rem] font-bold tracking-[-.04em]">Observability</h1>
        <p class="mt-1 text-[.78rem] text-muted">Cross-cutting view: traffic, latency, model spend, eval runs, workflows, bandit selection, hallucination scores.</p>
      </div>
    </section>
    <section class="mt-5">${metricCards(metrics)}</section>
    <section class="mt-5 grid gap-5 xl:grid-cols-[minmax(0,2fr)_minmax(280px,.85fr)]">
      <article class="panel p-5 sm:p-6"><div class="eyebrow">Top capabilities</div><div class="mt-3">${topCapabilitiesChart(topCaps)}</div></article>
      <article class="panel p-5 sm:p-6"><div class="eyebrow">API latency</div><div class="mt-3">${latencyBreakdown(metrics)}</div></article>
    </section>
  `;
  root.innerHTML = shell;
  return shell;
}
