// Eval run detail. Surfaces GET /api/v1/evals/{id} — the
// inputs, per-case scores, latency, and overall pass/fail.
import * as api from "../api.js";
import { escape, formatRelative, formatMoney, apiStatusLabel } from "../utils.js";

function pill(text, tone = "neutral") {
  return `<span class="status-pill ${tone} !px-2 !py-1"><span class="status-dot"></span>${escape(text)}</span>`;
}

function scoreTone(score) {
  if (typeof score !== "number") return "neutral";
  if (score >= 0.9) return "good";
  if (score >= 0.6) return "warn";
  return "danger";
}

export async function renderEvalDetail(route) {
  const root = window.document.getElementById("view");
  const id = route?.params?.id;
  if (!root) return "";
  if (!id) {
    root.innerHTML = `<p class="panel p-5 mt-5 text-[.78rem] text-muted">No eval id in the URL.</p>`;
    return;
  }
  root.innerHTML = `<section class="panel p-5"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
  const res = await api.getEval(id);
  if (!res.ok) {
    root.innerHTML = `<p class="panel p-5 mt-5 text-[.78rem] text-muted">Could not load eval: ${escape(apiStatusLabel(res))}</p>`;
    return;
  }
  const ev = res.data || {};
  const overall = typeof ev.overall_score === "number" ? ev.overall_score : (ev.score ?? null);
  const results = Array.isArray(ev.results) ? ev.results : (Array.isArray(ev.cases) ? ev.cases : []);
  const isPass = ev.status === "passed" || ev.status === "pass" || (overall != null && overall >= 0.6);
  root.innerHTML = `<section class="mt-5 space-y-5">
    <article class="panel p-5 sm:p-6">
      <div class="eyebrow flex items-center gap-2">
        <a href="#/capabilities" class="hover:text-ink">Capabilities</a>
        <span class="text-[#b2b3af]">/</span>
        ${ev.capability_id ? `<a href="#/capabilities/${escape(ev.capability_id)}" class="hover:text-ink">Capability</a><span class="text-[#b2b3af]">/</span>` : ""}
        ${ev.release_id ? `<a href="#/releases/${escape(ev.release_id)}" class="hover:text-ink">Release</a><span class="text-[#b2b3af]">/</span>` : ""}
        <span>Eval</span>
      </div>
      <div class="mt-2 flex items-center justify-between">
        <h1 class="text-[1.4rem] font-bold tracking-[-.04em] mono">${escape(ev.id || id)}</h1>
        ${pill(isPass ? "Pass" : (ev.status || (overall != null ? "Fail" : "—")), isPass ? "good" : (ev.status ? "danger" : "neutral"))}
      </div>
      <p class="mt-1 text-[.7rem] text-muted">${escape(formatRelative(ev.timestamp || ev.created_at))} · ${escape(ev.dataset_id || "—")} · ${escape(ev.scorer || "—")}</p>
      <dl class="mt-5 grid gap-3 sm:grid-cols-3">
        <div><dt class="eyebrow">Overall score</dt><dd class="mt-1 text-[1.1rem] font-bold">${overall != null ? overall.toFixed(3) : "—"}</dd></div>
        <div><dt class="eyebrow">Cases</dt><dd class="mt-1 text-[.78rem] font-bold mono">${results.length}</dd></div>
        <div><dt class="eyebrow">Latency</dt><dd class="mt-1 text-[.78rem] font-bold mono">${escape(String(ev.latency_ms || 0))}ms</dd></div>
      </dl>
      ${ev.error ? `<p class="mt-4 rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800 mono">${escape(ev.error)}</p>` : ""}
    </article>
    <article class="panel p-5 sm:p-6">
      <div class="eyebrow">Per-case results</div>
      ${renderCases(results)}
    </article>
  </section>`;
}

function renderCases(results) {
  if (!results.length) {
    return `<p class="mt-3 text-[.7rem] text-muted">No per-case details recorded.</p>`;
  }
  return `<table class="mt-2 w-full text-[.7rem]"><thead><tr class="text-left text-[.6rem] uppercase tracking-wider text-muted"><th class="py-1 font-bold">Case</th><th class="py-1 font-bold">Inputs</th><th class="py-1 font-bold">Expected</th><th class="py-1 font-bold">Score</th><th class="py-1 font-bold">Pass</th><th class="py-1 font-bold">Latency</th></tr></thead><tbody>${results.map((c, i) => `<tr class="border-t border-line/60">
    <td class="py-2 mono">#${i + 1}</td>
    <td class="py-2 mono text-[.62rem] truncate max-w-[16rem]">${escape(JSON.stringify(c.inputs || c.input || {}))}</td>
    <td class="py-2 mono text-[.62rem] truncate max-w-[16rem]">${escape(JSON.stringify(c.expected ?? "—"))}</td>
    <td class="py-2">${pill(typeof c.score === "number" ? c.score.toFixed(2) : "—", scoreTone(c.score))}</td>
    <td class="py-2">${pill(c.passed ? "Pass" : "Fail", c.passed ? "good" : "danger")}</td>
    <td class="py-2 mono">${escape(String(c.latency_ms || 0))}ms</td>
  </tr>`).join("")}</tbody></table>`;
}