// src/views/observability.js — cross-cutting observability page.
//
// Pulls the metrics summary + top-capabilities feed, surfaces them
// as KPI tiles + a latency breakdown + a usage bar chart.
//
// Migrated to ui.js primitives so the panel headers, metric cards,
// progress bars, and error states share a single source with the
// overview page.

import * as api from "../api.js";
import { escape, formatCompact, formatInteger, formatMoney, formatRelative } from "../utils.js";
import { pageHeader, panel, errorState, metricCard, metricGrid, progressBar } from "../ui.js";

function metricTiles(summary) {
  if (!summary || !summary.ok || !summary.data) {
    const msg = summary?.status === 401
      ? "Live counters unlock once an API key is set."
      : summary?.error ? `Unavailable (${summary.error})`
      : "Loading live counters…";
    return errorState({ ...summary, error: msg });
  }
  const apiM = summary.data.api_metrics || {};
  const llm = summary.data.llm_metrics || {};
  const review = summary.data.review_metrics || {};
  const guardrail = summary.data.guardrail_metrics || {};
  const evalMetrics = summary.data.eval_metrics || {};
  const workflow = summary.data.workflow_metrics || {};
  const errorRate = apiM.error_rate || 0;
  return metricGrid([
    metricCard({ eyebrow: "API requests", icon: "pulse", value: formatCompact(apiM.total_requests), sub: errorRate > 0 ? "Attention" : "Healthy", tone: errorRate > 0 ? "warn" : "good" }),
    metricCard({ eyebrow: "LLM spend", icon: "rocket", value: formatMoney(llm.total_cost_usd), sub: `${formatCompact(llm.total_calls)} calls`, tone: "neutral" }),
    metricCard({ eyebrow: "Reviews", icon: "scroll", value: formatInteger(review.total_reviews), sub: `${formatInteger(review.pending_count)} pending`, tone: review.pending_count > 0 ? "warn" : "good" }),
    metricCard({ eyebrow: "Guardrails", icon: "shield", value: formatInteger((guardrail.passes || 0) + (guardrail.blocks || 0)), sub: `${formatInteger(guardrail.blocks || 0)} blocks`, tone: guardrail.blocks > 0 ? "warn" : "good" }),
    metricCard({ eyebrow: "Eval runs", icon: "flask", value: formatInteger(evalMetrics.total_runs || 0), sub: `${formatInteger(evalMetrics.total_cases || 0)} cases`, tone: "neutral" }),
    metricCard({ eyebrow: "Workflows", icon: "grid", value: formatInteger(workflow.total_runs || 0), sub: `${formatInteger(workflow.active_count || 0)} active`, tone: workflow.active_count > 0 ? "good" : "neutral" }),
    metricCard({ eyebrow: "Bandit", icon: "rocket", value: formatInteger(summary.data.bandit_metrics?.selections_total || 0), sub: `run ${summary.data.bandit_metrics?.current_run_id || "—"}`, tone: "neutral" }),
    metricCard({ eyebrow: "Hallucination", icon: "warning", value: typeof summary.data.hallucination_metrics?.avg_score === "number" ? summary.data.hallucination_metrics.avg_score.toFixed(2) : "—", sub: `p95 ${typeof summary.data.hallucination_metrics?.p95_score === "number" ? summary.data.hallucination_metrics.p95_score.toFixed(2) : "—"}`, tone: "neutral" }),
  ]);
}

function latencyRow(label, ms, total) {
  const pct = total > 0 ? (ms / total) : 0;
  // Re-use progressBar but render as ms-only display.
  return `<div>
    <div class="flex items-center justify-between"><span class="text-[.7rem] font-semibold">${escape(label)}</span><span class="mono text-[.66rem] text-muted">${escape(ms.toFixed(2))}ms</span></div>
    <div class="mt-1 h-2 overflow-hidden rounded-full bg-paper"><div class="h-full rounded-full bg-[#789c35]" style="width:${Math.min(100, pct).toFixed(1)}%"></div></div>
  </div>`;
}

function latencyBreakdown(summary) {
  if (!summary || !summary.ok || !summary.data) {
    return `<p class="text-[.66rem] text-muted">Latency breakdown unavailable.</p>`;
  }
  const apiM = summary.data.api_metrics || {};
  const total = (apiM.p50_latency_ms || 0) + (apiM.p95_latency_ms || 0) + (apiM.p99_latency_ms || 0);
  return `<div class="space-y-3">${latencyRow("p50", apiM.p50_latency_ms || 0, total)}${latencyRow("p95", apiM.p95_latency_ms || 0, total)}${latencyRow("p99", apiM.p99_latency_ms || 0, total)}</div>`;
}

function topCapabilitiesChart(topCaps) {
  if (!topCaps || !topCaps.ok || !Array.isArray(topCaps.data?.capabilities) || !topCaps.data.capabilities.length) {
    const text = topCaps?.status === 429
      ? "Top capabilities feed rate-limited. Retrying."
      : !topCaps ? "Loading…" : "No recorded capability calls yet.";
    return `<p class="text-[.66rem] text-muted">${escape(text)}</p>`;
  }
  const rows = topCaps.data.capabilities.slice(0, 10);
  const maxCount = Math.max(...rows.map((r) => r.count || 0), 1);
  return `<div class="space-y-3">${rows.map((cap) => {
    const count = cap.count || 0;
    const width = (count / maxCount);
    return `<div>
      <div class="flex items-center justify-between"><a href="#/capabilities/${escape(cap.id || "")}" class="text-[.7rem] font-bold text-ink truncate max-w-[60%]">${escape(cap.name || cap.id || "Unknown")}</a><span class="mono text-[.66rem] text-muted">${formatCompact(count)} calls</span></div>
      <div class="mt-1 h-2 overflow-hidden rounded-full bg-paper"><div class="h-full rounded-full bg-[#6878ff]" style="width:${(width * 100).toFixed(1)}%"></div></div>
      ${typeof cap.avg_latency_ms === "number" ? `<div class="mt-0.5 flex justify-between text-[.58rem] text-muted"><span>${formatRelative(cap.last_used)}</span><span>avg ${escape(Math.round(cap.avg_latency_ms))}ms</span></div>` : ""}
    </div>`;
  }).join("")}</div>`;
}

export async function renderObservability(route) {
  const root = window.document.getElementById("view");
  if (!root) return "";
  root.innerHTML = `<section class="panel p-6"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
  const [metrics, topCaps] = await api.sequential([
    () => api.getMetricsSummary(),
    () => api.getTopCapabilities(),
  ], { delayMs: 60 });

  const html = [
    pageHeader({
      eyebrow: "Operations",
      title: "Observability",
      description: "Cross-cutting view: traffic, latency, model spend, eval runs, workflows, bandit selection, hallucination scores.",
    }),
    `<section class="mt-5">${metricTiles(metrics)}</section>`,
    `<section class="mt-5 grid gap-5 xl:grid-cols-[minmax(0,2fr)_minmax(280px,.85fr)]">
      ${panel({ eyebrow: "Top capabilities", body: topCapabilitiesChart(topCaps) })}
      ${panel({ eyebrow: "API latency", body: latencyBreakdown(metrics) })}
    </section>`,
  ].join("");
  root.innerHTML = html;
  return html;
}
