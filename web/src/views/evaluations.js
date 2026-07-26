import * as api from "../api.js";
import { escape, formatRelative, formatPercent, apiStatusLabel } from "../utils.js";

function pill(text, tone = "neutral") {
  return `<span class="status-pill ${tone}"><span class="status-dot"></span>${escape(text)}</span>`;
}

function statusTone(status) {
  return ({ success: "good", ok: "good", completed: "good", running: "warn", failed: "danger", error: "danger" })[status] || "neutral";
}

function executionRow(exec) {
  return `<tr class="border-t border-line/60">
    <td class="py-2 pr-3 text-[.66rem] text-muted whitespace-nowrap">${escape(formatRelative(exec.created_at))}</td>
    <td class="py-2 pr-3 mono text-[.66rem] truncate max-w-[12rem]">${escape(exec.id)}</td>
    <td class="py-2 pr-3">${pill(exec.status || "—", statusTone(exec.status))}</td>
    <td class="py-2 pr-3 mono text-[.66rem]">${escape(exec.input_hash?.slice(0, 12) || "—")}…</td>
    <td class="py-2 pr-3 mono text-[.66rem]">${escape(exec.manifest_hash?.slice(0, 12) || "—")}…</td>
    <td class="py-2 pr-3 mono text-[.66rem]">${escape(exec.model || "—")}</td>
    <td class="py-2 pr-3 mono text-[.66rem]">${typeof exec.latency_ms === "number" ? `${Math.round(exec.latency_ms)}ms` : "—"}</td>
  </tr>`;
}

function renderTable(executions) {
  if (!executions || !executions.ok || !executions.data || !executions.data.length) {
    const tone = !executions || executions.ok === false ? (executions?.status === 429 ? "warn" : "neutral") : "neutral";
    const text = executions?.status === 429 ? "Executions feed rate-limited. Retrying." : !executions ? "Loading…" : "No executions recorded for this version yet.";
    return `<p class="text-[.66rem] text-muted">${escape(text)}</p>`;
  }
  return `<div class="overflow-x-auto rounded-xl border border-line"><table class="w-full text-[.7rem]"><thead><tr class="bg-paper text-left text-[.6rem] uppercase tracking-wider text-muted"><th class="px-3 py-2 font-bold">When</th><th class="px-3 py-2 font-bold">Execution id</th><th class="px-3 py-2 font-bold">Status</th><th class="px-3 py-2 font-bold">Input hash</th><th class="px-3 py-2 font-bold">Manifest hash</th><th class="px-3 py-2 font-bold">Model</th><th class="px-3 py-2 font-bold">Latency</th></tr></thead><tbody>${executions.data.map(executionRow).join("")}</tbody></table></div>
  <p class="mt-3 text-[.62rem] text-muted">Showing ${executions.data.length} executions.</p>`;
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
    root.innerHTML = `<p class="panel p-6 text-center text-[.78rem]">${escape(apiStatusLabel(workspaces))}</p>`;
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

  const shell = `
    <section class="flex flex-wrap items-end justify-between gap-3">
      <div>
        <div class="eyebrow">Runtime</div>
        <h1 class="mt-2 text-[1.4rem] font-bold tracking-[-.04em]">Evaluations</h1>
        <p class="mt-1 text-[.78rem] text-muted">Live executions for capability versions. Pick a version to see history; the right pane runs a fresh execution against the same manifest.</p>
      </div>
    </section>
    <section class="mt-5 grid gap-5 xl:grid-cols-[minmax(0,2fr)_minmax(320px,1fr)]">
      <article class="panel p-5 sm:p-6">
        <div class="flex items-center justify-between">
          <div><div class="eyebrow">Executions</div>${selected ? `<h2 class="mt-1 text-[1rem] font-bold">${escape(selected.name)} v${escape(selected.version)}</h2>` : ""}</div>
          ${selected ? `<select id="exec-version" class="field !h-9 !rounded-lg !w-72 !text-[.72rem]">${versions.map(versionOption).join("")}</select>` : ""}
        </div>
        <div class="mt-5" id="exec-table">${renderTable(executions)}</div>
      </article>
      <article class="panel p-5 sm:p-6">
        <div class="eyebrow">Run</div>
        ${selected ? `<form id="run-form" class="mt-3 space-y-3">
          <div><label class="eyebrow mb-2 block" for="run-inputs">Inputs (JSON)</label><textarea id="run-inputs" name="inputs" class="field mono min-h-32 resize-y" placeholder='{"q": "hello"}'>{"q": "hello"}</textarea></div>
          <p id="run-error" class="hidden rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800"></p>
          <p id="run-success" class="hidden rounded-lg bg-lime/15 px-3 py-2 text-[.68rem] text-[#52632d]"></p>
          <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
            <button type="button" id="run-cancel" class="quiet-button hidden">Close</button>
            <button type="submit" class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-play"/></svg>Run</button>
          </div>
        </form>` : `<p class="mt-3 text-[.7rem] text-muted">Create a version first; executions require a manifest.</p>`}
      </article>
    </section>
  `;
  root.innerHTML = shell;

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
    success.classList.add("hidden");
    let inputs = null;
    try {
      inputs = JSON.parse(root.querySelector("#run-inputs").value);
    } catch (e) {
      error.textContent = `Invalid JSON in inputs: ${e.message}`;
      error.classList.remove("hidden");
      return;
    }
    const cancel = root.querySelector("#run-cancel");
    cancel.classList.remove("hidden");
    const submit = form.querySelector("button[type=submit]");
    submit.disabled = true;
    const result = await api.createExecution(selected.id, { inputs });
    submit.disabled = false;
    if (!result.ok) {
      error.textContent = apiStatusLabel(result);
      error.classList.remove("hidden");
      return;
    }
    success.textContent = `Execution ${result.data.id} accepted (status ${result.data.status || "queued"}).`;
    success.classList.remove("hidden");
    window.location.hash = `#/evaluations?version=${encodeURIComponent(selected.id)}`;
    window.location.reload();
  });
  root.querySelector("#run-cancel")?.addEventListener("click", () => window.location.reload());
  return shell;
}
