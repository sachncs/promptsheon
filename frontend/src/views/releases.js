// src/views/releases.js — releases listing page.
//
// Surfaces every release across every workspace / project / capability
// in a single flat table with environment and status filters. Uses
// ui.js primitives so the page header, filter chips, table, and
// status pills match the rest of the dashboard.

import * as api from "../api.js";
import { escape, formatRelative } from "../utils.js";
import { statusPill, pageHeader, panel, chipGroup, dataTable, emptyState, errorState, badge } from "../ui.js";

const ENV_TONES = { prod: "good", staging: "warn", dev: "neutral" };
const STATUS_TONES = { active: "good", approved: "good", pending: "warn", superseded: "neutral", rolled_back: "danger" };
const ENVS = ["prod", "staging", "dev"];
const STATUS_OPTIONS = ["active", "approved", "pending", "superseded", "rolled_back"];

function envTone(env) { return ENV_TONES[env] || "neutral"; }
function statusTone(status) { return STATUS_TONES[status] || "neutral"; }

function rowRenderer(capMap) {
  return (release) => {
    const capName = capMap.get(release.capability_id)?.name || "Unknown";
    return {
      when: `<span class="text-muted whitespace-nowrap">${escape(formatRelative(release.created_at))}</span>`,
      capability: `<button type="button" data-open-release="${escape(release.id)}" class="text-[.7rem] font-bold text-left hover:underline">${escape(capName)}</button>`,
      version: `<span class="mono text-[.66rem]">v${escape(release.capability_version)}</span>`,
      environment: statusPill(release.environment, envTone(release.environment)),
      status: statusPill(release.status, statusTone(release.status)),
      id: `<span class="text-[.62rem] text-muted mono truncate max-w-[12rem] inline-block align-middle" title="${escape(release.id)}">${escape(release.id)}</span>`,
    };
  };
}

export async function renderReleases(route) {
  const root = window.document.getElementById("view");
  if (!root) return "";
  const envFilter = route?.query?.env || "all";
  const statusFilter = route?.query?.status || "all";
  root.innerHTML = `<section class="panel p-6"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-10 w-full"></div></section>`;

  const workspaces = await api.listWorkspaces();
  if (!workspaces.ok) {
    root.innerHTML = `<p class="panel p-6 text-center text-[.78rem] text-muted">${escape(workspaces.error || "Failed to load workspaces.")}</p>`;
    return "";
  }

  const allProjects = [];
  const allCaps = [];
  const allRels = [];
  for (const ws of workspaces.data || []) {
    const projects = await api.listProjects(ws.id);
    if (projects.ok) for (const p of projects.data || []) allProjects.push(p);
    await new Promise((r) => setTimeout(r, 60));
  }
  for (const p of allProjects) {
    const caps = await api.listCapabilities(p.id);
    if (caps.ok) for (const c of caps.data || []) allCaps.push(c);
    await new Promise((r) => setTimeout(r, 60));
  }
  for (const c of allCaps) {
    const rels = await api.listReleases(c.id);
    if (rels.ok) for (const r of rels.data || []) allRels.push(r);
    await new Promise((r) => setTimeout(r, 60));
  }

  const capMap = new Map(allCaps.map((c) => [c.id, c]));
  const filtered = allRels.filter((r) =>
    (envFilter === "all" || r.environment === envFilter) &&
    (statusFilter === "all" || r.status === statusFilter),
  ).sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());

  const envItems = [{ key: "all", label: "all envs" }, ...ENVS.map((e) => ({ key: e, label: e }))];
  const statusItems = [{ key: "all", label: "all" }, ...STATUS_OPTIONS.map((s) => ({ key: s, label: s.replace("_", " ") }))];

  const table = dataTable({
    columns: [
      { key: "when", label: "When" },
      { key: "capability", label: "Capability" },
      { key: "version", label: "Version" },
      { key: "environment", label: "Environment" },
      { key: "status", label: "Status" },
      { key: "id", label: "Release id" },
    ],
    rows: filtered.map((r) => rowRenderer(capMap)(r)),
    emptyMessage: "No releases match this filter.",
    emptyIcon: "icon-rocket",
  });

  const html = [
    pageHeader({
      eyebrow: "Release pipeline",
      title: `Releases (${allRels.length} total · ${filtered.length} shown)`,
      description: "Cross-workspace view of every release. Click a row to vote or rollback in the modal.",
    }),
    panel({
      eyebrow: "Filter",
      title: "Find releases",
      body: `
        <div class="flex flex-wrap items-end gap-3">
          <div><div class="field-label">Environment</div>${chipGroup(envItems, { activeKey: envFilter, onClickDataAttr: "data-release-env" })}</div>
          <div><div class="field-label">Status</div>${chipGroup(statusItems, { activeKey: statusFilter, onClickDataAttr: "data-release-status" })}</div>
        </div>
      `,
    }),
    panel({ title: "Releases", rightSlot: badge(`${filtered.length}`, { tone: "neutral" }), body: table, padded: false }),
  ].join("");
  root.innerHTML = html;

  root.querySelectorAll("[data-release-env]").forEach((b) => {
    b.addEventListener("click", () => {
      const next = b.dataset.releaseEnv;
      const params = new URLSearchParams();
      if (next !== "all") params.set("env", next);
      if (statusFilter !== "all") params.set("status", statusFilter);
      window.location.hash = `#/releases${params.toString() ? `?${params}` : ""}`;
      window.location.reload();
    });
  });
  root.querySelectorAll("[data-release-status]").forEach((b) => {
    b.addEventListener("click", () => {
      const next = b.dataset.releaseStatus;
      const params = new URLSearchParams();
      if (envFilter !== "all") params.set("env", envFilter);
      if (next !== "all") params.set("status", next);
      window.location.hash = `#/releases${params.toString() ? `?${params}` : ""}`;
      window.location.reload();
    });
  });
  root.querySelectorAll("[data-open-release]").forEach((b) => {
    b.addEventListener("click", async () => {
      const id = b.dataset.openRelease;
      const mod = await import("./release-modal.js");
      if (typeof mod.openReleaseModal === "function") mod.openReleaseModal(id);
    });
  });

  return html;
}
