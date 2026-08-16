// Eval run detail. Surfaces GET /api/v1/evals/{id} — the inputs,
// per-case scores, latency, and overall pass/fail.

import * as api from "../api.js";
import { escape, formatRelative, apiStatusLabel } from "../utils.js";
import { statusPill, pageHeader, panel, dataTable, emptyState, errorState, inlineBanner } from "../ui.js";

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
    root.innerHTML = `<div class="mt-5">${errorState({ ok: false, error: "No eval id in the URL." })}</div>`;
    return;
  }
  root.innerHTML = `<section class="panel p-5"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
  const res = await api.getEval(id);
  if (!res.ok) {
    root.innerHTML = `<div class="mt-5">${errorState({ ...res, error: `Could not load eval: ${apiStatusLabel(res)}` })}</div>`;
    return;
  }
  const ev = res.data || {};
  const overall = typeof ev.overall_score === "number" ? ev.overall_score : (ev.score ?? null);
  const results = Array.isArray(ev.results) ? ev.results : (Array.isArray(ev.cases) ? ev.cases : []);
  const isPass = ev.status === "passed" || ev.status === "pass" || (overall != null && overall >= 0.6);
  const passTone = isPass ? "good" : ev.status ? "danger" : "neutral";
  const passLabel = isPass ? "Pass" : (ev.status || (overall != null ? "Fail" : "—"));

  const breadcrumbs = `<nav class="eyebrow flex items-center gap-2" aria-label="Breadcrumb">
    <a href="#/capabilities" class="hover:text-ink">Capabilities</a>
    <span class="text-[#b2b3af]">/</span>
    ${ev.capability_id ? `<a href="#/capabilities/${escape(ev.capability_id)}" class="hover:text-ink">Capability</a><span class="text-[#b2b3af]">/</span>` : ""}
    ${ev.release_id ? `<a href="#/releases/${escape(ev.release_id)}" class="hover:text-ink">Release</a><span class="text-[#b2b3af]">/</span>` : ""}
    <span>Eval</span>
  </nav>`;

  const casesTable = results.length ? dataTable({
    columns: [
      { key: "case", label: "Case", render: (_c, i) => `<span class="mono">#${i + 1}</span>` },
      { key: "inputs", label: "Inputs", render: (c) => `<span class="mono truncate max-w-[16rem] inline-block align-middle">${escape(JSON.stringify(c.inputs || c.input || {}))}</span>` },
      { key: "expected", label: "Expected", render: (c) => `<span class="mono truncate max-w-[16rem] inline-block align-middle">${escape(JSON.stringify(c.expected ?? "—"))}</span>` },
      { key: "score", label: "Score", render: (c) => statusPill(typeof c.score === "number" ? c.score.toFixed(2) : "—", scoreTone(c.score)) },
      { key: "pass", label: "Pass", render: (c) => statusPill(c.passed ? "Pass" : "Fail", c.passed ? "good" : "danger") },
      { key: "latency", label: "Latency", render: (c) => `<span class="mono">${escape(String(c.latency_ms || 0))}ms</span>` },
    ],
    rows: results,
    emptyMessage: "No per-case details.",
    emptyIcon: "icon-flask",
  }) : emptyState("No per-case details recorded.", { icon: "icon-flask" });

  root.innerHTML = `${pageHeader({
    eyebrow: breadcrumbs,
    title: ev.id || id,
    description: `${formatRelative(ev.timestamp || ev.created_at)} · ${ev.dataset_id || "—"} · ${ev.scorer || "—"}`,
    actions: statusPill(passLabel, passTone),
  })}
  <section class="mt-5 space-y-5">
    ${panel({
      eyebrow: "Eval metadata",
      title: "Telemetry",
      body: `
        <dl class="grid gap-3 sm:grid-cols-3">
          <div><dt class="eyebrow">Overall score</dt><dd class="mt-1 text-[1.1rem] font-bold">${overall != null ? overall.toFixed(3) : "—"}</dd></div>
          <div><dt class="eyebrow">Cases</dt><dd class="mt-1 text-[.78rem] font-bold mono">${results.length}</dd></div>
          <div><dt class="eyebrow">Latency</dt><dd class="mt-1 text-[.78rem] font-bold mono">${escape(String(ev.latency_ms || 0))}ms</dd></div>
        </dl>
        ${ev.error ? inlineBanner({ tone: "danger", message: ev.error }) : ""}
      `,
    })}
    ${panel({ eyebrow: "Per-case results", title: "Breakdown", body: casesTable })}
  </section>`;
}
