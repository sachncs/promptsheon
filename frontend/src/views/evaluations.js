// src/views/evaluations.js — eval / execution view.
//
// Lets the operator pick a capability version, see the execution
// history, and submit a fresh execution against the same manifest.
// The view uses ui.js primitives for the page header, panels, data
// table, status pills, and run-form banners.

import * as api from "../api.js";
import { escape, formatRelative, apiStatusLabel } from "../utils.js";
import { statusPill, pageHeader, panel, dataTable, emptyState, errorState, inlineBanner } from "../ui.js";

const STATUS_TONES = { success: "good", ok: "good", completed: "good", passed: "good", pass: "good", running: "warn", pending: "warn", failed: "danger", error: "danger" };
function statusTone(s) { return STATUS_TONES[s] || "neutral"; }
function scoreTone(overall, status) {
  if (status === "passed" || status === "pass") return "good";
  if (overall != null && overall >= 0.6) return "warn";
  return "danger";
}

function executionRow(exec) {
  return {
    when: `<span class="text-muted whitespace-nowrap">${escape(formatRelative(exec.created_at))}</span>`,
    id: `<span class="mono truncate max-w-[12rem] inline-block align-middle">${escape(exec.id)}</span>`,
    status: statusPill(exec.status || "—", statusTone(exec.status)),
    input: `<span class="mono truncate max-w-[12rem] inline-block align-middle">${escape(exec.input_hash?.slice(0, 12) || "—")}…</span>`,
    manifest: `<span class="mono truncate max-w-[12rem] inline-block align-middle">${escape(exec.manifest_hash?.slice(0, 12) || "—")}…</span>`,
    model: `<span class="mono truncate max-w-[10rem] inline-block align-middle">${escape(exec.model || "—")}</span>`,
    latency: `<span class="mono">${typeof exec.latency_ms === "number" ? `${Math.round(exec.latency_ms)}ms` : "—"}</span>`,
  };
}

function evalRow(ev) {
  const overall = typeof ev.overall_score === "number" ? ev.overall_score : (ev.score ?? null);
  const tone = scoreTone(overall, ev.status);
  return {
    when: `<span class="text-muted whitespace-nowrap">${escape(formatRelative(ev.timestamp || ev.created_at))}</span>`,
    id: `<a href="#/evals/${escape(ev.id)}" class="mono truncate max-w-[12rem] inline-block align-middle hover:underline">${escape(ev.id)}</a>`,
    score: statusPill(overall != null ? overall.toFixed(2) : "—", tone),
    dataset: `<span class="truncate max-w-[10rem] inline-block align-middle">${escape(ev.dataset_id || "—")}</span>`,
    scorer: `<span class="truncate max-w-[10rem] inline-block align-middle">${escape(ev.scorer || "—")}</span>`,
    latency: `<span class="mono">${escape(String(ev.latency_ms || 0))}ms</span>`,
  };
}

function versionOption(v) {
  return `<option value="${escape(v.id)}">${escape(v.name)} v${escape(v.version)}</option>`;
}

export async function renderEvaluations(route) {
  const root = window.document.getElementById("view");
  if (!root) return "";
  root.innerHTML = `<section class="panel p-6"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
  const versionId = route?.query?.version || "";

  const workspaces = await api.listWorkspaces();
  if (!workspaces.ok) {
    root.innerHTML = `<div class="panel p-6">${errorState(workspaces)}</div>`;
    return "";
  }

  const versions = [];
  for (const ws of workspaces.data || []) {
    const projects = await api.listProjects(ws.id);
    if (!projects.ok) continue;
    for (const p of projects.data || []) {
      const caps = await api.listCapabilities(p.id);
      if (!caps.ok) continue;
      for (const c of caps.data || []) {
        const vs = await api.listVersions(c.id);
        if (vs.ok) for (const v of vs.data || []) versions.push({ id: v.id, name: `${c.name}`, version: v.version });
      }
    }
  }
  versions.sort((a, b) => a.name.localeCompare(b.name));

  const selected = versions.find((v) => v.id === versionId) || versions[0];
  const executions = selected ? await api.listExecutions(selected.id) : { ok: false };
  const evals = selected ? await api.listEvals(selected.id).catch(() => ({ ok: false })) : { ok: false };

  const execPanel = panel({
    eyebrow: "Executions",
    title: selected ? `${selected.name} v${selected.version}` : "No version selected",
    rightSlot: selected ? `<select id="exec-version" class="field !h-9 !rounded-lg !w-72 !text-[.72rem]" aria-label="Pick a version">${versions.map(versionOption).join("")}</select>` : "",
    body: `
      <div id="exec-table">
        ${executions?.ok && executions.data?.length ? dataTable({
          columns: [
            { key: "when", label: "When" },
            { key: "id", label: "Execution id" },
            { key: "status", label: "Status" },
            { key: "input", label: "Input hash" },
            { key: "manifest", label: "Manifest hash" },
            { key: "model", label: "Model" },
            { key: "latency", label: "Latency" },
          ],
          rows: executions.data.map(executionRow),
          emptyMessage: "No executions recorded for this version yet.",
          emptyIcon: "icon-play",
        }) + `<p class="mt-3 text-[.62rem] text-muted">Showing ${executions.data.length} executions.</p>`
        : (executions?.status === 429 ? inlineBanner({ tone: "warn", message: "Executions feed rate-limited. Retrying." }) : emptyState("No executions recorded for this version yet.", { icon: "icon-play" }))}
      </div>
      ${evals?.ok && evals.data?.length ? dataTable({
        columns: [
          { key: "when", label: "When" },
          { key: "id", label: "Eval id" },
          { key: "score", label: "Score" },
          { key: "dataset", label: "Dataset" },
          { key: "scorer", label: "Scorer" },
          { key: "latency", label: "Latency" },
        ],
        rows: evals.data.map(evalRow),
        emptyMessage: "No eval runs.",
        emptyIcon: "icon-flask",
      }) + `<p class="mt-3 text-[.62rem] text-muted">Showing ${evals.data.length} eval runs.</p>`
      : ""}
    `,
  });

  const runPanel = panel({
    eyebrow: "Run",
    title: "Submit an execution",
    body: selected ? `
      <form id="run-form" class="space-y-3">
        <div>
          <label class="field-label" for="run-inputs">Inputs (JSON)</label>
          <textarea id="run-inputs" name="inputs" class="field mono min-h-32 resize-y" placeholder='{"q": "hello"}'>{"q": "hello"}</textarea>
        </div>
        <p id="run-error" class="hidden"></p>
        <p id="run-success" class="hidden"></p>
        <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
          <button type="button" id="run-cancel" class="quiet-button hidden">Close</button>
          <button type="submit" class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-play"/></svg>Run</button>
        </div>
      </form>
    ` : `<p class="mt-3 text-[.7rem] text-muted">Create a version first; executions require a manifest.</p>`,
  });

  const html = [
    pageHeader({
      eyebrow: "Runtime",
      title: "Evaluations",
      description: "Live executions for capability versions. Pick a version to see history; the right pane runs a fresh execution against the same manifest.",
    }),
    `<section class="mt-5 grid gap-5 xl:grid-cols-[minmax(0,2fr)_minmax(320px,1fr)]">${execPanel}${runPanel}</section>`,
  ].join("");

  root.innerHTML = html;

  root.querySelector("#exec-version")?.addEventListener("change", (event) => {
    window.location.hash = `#/evaluations?version=${encodeURIComponent(event.target.value)}`;
    window.location.reload();
  });

  const form = root.querySelector("#run-form");
  form?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const error = root.querySelector("#run-error");
    const success = root.querySelector("#run-success");
    error.classList.add("hidden");
    error.innerHTML = "";
    success.classList.add("hidden");
    success.innerHTML = "";
    let inputs = null;
    try {
      inputs = JSON.parse(root.querySelector("#run-inputs").value);
    } catch (e) {
      error.innerHTML = inlineBanner({ tone: "danger", message: `Invalid JSON in inputs: ${e.message}` });
      error.classList.remove("hidden");
      return;
    }
    const cancel = root.querySelector("#run-cancel");
    cancel.classList.remove("hidden");
    const submit = form.querySelector('button[type="submit"]');
    submit.disabled = true;
    const result = await api.createExecution(selected.id, { inputs });
    submit.disabled = false;
    if (!result.ok) {
      error.innerHTML = inlineBanner({ tone: "danger", message: apiStatusLabel(result) });
      error.classList.remove("hidden");
      return;
    }
    success.innerHTML = inlineBanner({ tone: "good", message: `Execution ${result.data.id} accepted (status ${result.data.status || "queued"}).` });
    success.classList.remove("hidden");
    window.location.hash = `#/evaluations?version=${encodeURIComponent(selected.id)}`;
    window.location.reload();
  });
  root.querySelector("#run-cancel")?.addEventListener("click", () => window.location.reload());
  return html;
}
