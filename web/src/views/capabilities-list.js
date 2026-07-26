import * as api from "../api.js";
import { escape, formatCompact, formatInteger, formatMoney, formatRelative } from "../utils.js";
import { ownerName } from "../state/owners.js";

function pill(text, tone = "neutral") {
  return `<span class="status-pill ${tone}"><span class="status-dot"></span>${escape(text)}</span>`;
}

function statusFor(release) {
  if (!release) return { label: "Draft", tone: "neutral" };
  const map = { active: ["Production", "good"], approved: ["Approved", "good"], pending: ["Pending", "warn"], superseded: ["Superseded", "neutral"], rolled_back: ["Rolled back", "danger"] };
  const [label, tone] = map[release.status] || [release.status, "neutral"];
  return { label, tone };
}

function renderTable(capabilities, releases, projects) {
  if (!capabilities?.length) return `<div class="panel p-8 text-center"><p class="text-[.78rem] font-bold">No capabilities yet</p><p class="mt-1 text-[.66rem] text-muted">Once a project has a capability, it shows up here. Use the New capability button on the Overview to create the first.</p></div>`;
  const projectMap = new Map((projects || []).map((p) => [p.id, p.name]));
  const byCap = new Map();
  for (const r of releases || []) {
    if (!byCap.has(r.capability_id)) byCap.set(r.capability_id, []);
    byCap.get(r.capability_id).push(r);
  }
  return `<div class="panel overflow-hidden">
    <div class="hidden grid-cols-[minmax(220px,1.7fr)_110px_120px_130px_120px_32px] gap-4 border-b border-line/70 bg-paper/70 px-5 py-3 text-[.62rem] font-bold uppercase tracking-[.12em] text-muted md:grid"><span>Capability</span><span>Version</span><span>Status</span><span>Reliability</span><span>Owner</span><span></span></div>
    ${capabilities.map((cap) => {
      const rels = byCap.get(cap.id) || [];
      const release = rels.find((r) => r.status === "active") || rels.find((r) => r.status === "pending") || rels[0];
      const version = rels.reduce((m, r) => Math.max(m, r.capability_version || 0), 0) || 1;
      const s = statusFor(release);
      return `<a href="#/capabilities/${escape(cap.id)}" data-searchable class="data-row grid gap-3 px-5 py-4 md:grid-cols-[minmax(220px,1.7fr)_110px_120px_130px_120px_32px] md:items-center md:gap-4">
        <div class="flex min-w-0 items-center gap-3">
          <span class="grid h-9 w-9 shrink-0 place-items-center rounded-[10px] bg-[#e9ecff] text-blue"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-spark"/></svg></span>
          <span class="min-w-0"><span class="block truncate text-[.78rem] font-bold">${escape(cap.name)}</span><span class="mt-1 block truncate text-[.64rem] text-muted">${escape(projectMap.get(cap.project_id) || "Unknown project")} · ${escape(cap.description || "No description")}</span></span>
        </div>
        <span class="mono text-[.68rem] text-[#62656a]">v${escape(version)}</span>
        <span>${pill(s.label, s.tone)}</span>
        <span class="text-[.74rem] font-bold">— <span class="block text-[.62rem] font-medium text-muted">no evals yet</span></span>
        <span class="text-[.72rem] text-[#686b70]">${escape(ownerName(cap.owner))}</span>
        <span class="icon-button !h-7 !w-7 !border-0 !bg-transparent"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.8]"><use href="#icon-arrow-right"/></svg></span>
      </a>`;
    }).join("")}
  </div>`;
}

export async function renderCapabilitiesList(route) {
  const root = window.document.getElementById("view");
  if (!root) return "";
  const skeleton = `<section class="panel p-6"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
  root.innerHTML = skeleton;
  const workspacesRes = await api.listWorkspaces();
  if (!workspacesRes.ok) {
    const fallback = `<p class="panel p-6 text-center text-[.78rem]">${escape(workspacesRes.error || "API error")}</p>`;
    root.innerHTML = fallback;
    return fallback;
  }
  const workspaces = workspacesRes.data || [];
  const allProjects = [];
  const allCaps = [];
  const allRels = [];
  for (const ws of workspaces) {
    const projects = await api.listProjects(ws.id);
    if (!projects.ok) continue;
    for (const p of projects.data || []) allProjects.push(p);
  }
  for (const p of allProjects) {
    const r = await api.listCapabilities(p.id);
    if (!r.ok) continue;
    for (const c of r.data || []) allCaps.push(c);
  }
  for (const c of allCaps) {
    const r = await api.listReleases(c.id);
    if (!r.ok) continue;
    for (const rel of r.data || []) allRels.push(rel);
  }
  const html = `
    <section class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <div class="eyebrow">Workspace catalog</div>
        <h1 class="mt-2 text-[1.4rem] font-bold tracking-[-.04em]">All capabilities (${allCaps.length})</h1>
        <p class="mt-1 text-[.78rem] text-muted">Cross-project view. Click any row to manage versions, contract, self-evolve, and reputation.</p>
      </div>
      <div class="flex items-center gap-2">
        <input id="cap-search" placeholder="Filter by name, project, owner" class="field !h-9 !w-64 !rounded-lg !border-line !bg-white/65 !text-[.72rem]" />
      </div>
    </section>
    <div id="cap-table" class="mt-5">${renderTable(allCaps, allRels, allProjects)}</div>`;
  root.innerHTML = html;
  const search = root.querySelector("#cap-search");
  search?.addEventListener("input", () => {
    const q = search.value.trim().toLowerCase();
    root.querySelectorAll("[data-searchable]").forEach((row) => {
      row.hidden = q.length > 0 && !row.textContent.toLowerCase().includes(q);
    });
  });
  return "";
}
