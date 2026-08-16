// Catalog view. Cross-workspace capability search via
// GET /api/v1/catalog/capabilities. Reached via /operations/catalog.
// Operator picks a workspace + query string and gets a flat list of
// matching capabilities with a link into the capability-detail page.

import * as api from "../api.js";
import { escape, formatRelative, apiStatusLabel } from "../utils.js";
import { statusPill, pageHeader, panel, dataTable, emptyState, errorState } from "../ui.js";

export async function renderCatalogTab(root) {
  if (!root) return "";
  root.innerHTML = `<section class="panel p-5"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
  const wsRes = await api.listWorkspaces();
  if (!wsRes.ok) {
    root.innerHTML = `<div class="mt-5">${errorState({ ...wsRes, error: `Workspaces unavailable: ${apiStatusLabel(wsRes)}` })}</div>`;
    return;
  }
  const workspaces = wsRes.data || [];

  root.innerHTML = `${pageHeader({
    eyebrow: "Cross-workspace catalog",
    title: "Search",
    description: "Catalog search spans every workspace. The dropdown picks the workspace; the input filters by name substring.",
  })}
  <section class="mt-5 space-y-5">
    ${panel({
      eyebrow: "Filter",
      title: "Workspace + query",
      body: `
        <form id="catalog-form" class="grid gap-3 sm:grid-cols-[minmax(220px,1fr)_1fr_auto]">
          <select id="catalog-ws" name="workspace_id" class="field" aria-label="Workspace">${workspaces.map((w, i) => `<option value="${escape(w.id)}" ${i === 0 ? "selected" : ""}>${escape(w.name)}</option>`).join("")}</select>
          <input id="catalog-q" name="q" class="field" placeholder="substring filter (optional)" />
          <button type="submit" class="primary-button">Search</button>
        </form>
        <div id="catalog-results" class="mt-5"></div>
      `,
    })}
  </section>`;

  root.querySelector("#catalog-form")?.addEventListener("submit", (event) => {
    event.preventDefault();
    runCatalogSearch(root);
  });

  if (workspaces.length) {
    root.querySelector("#catalog-ws").value = workspaces[0].id;
    await runCatalogSearch(root);
  }
}

async function runCatalogSearch(root) {
  const ws = root.querySelector("#catalog-ws").value;
  const q = root.querySelector("#catalog-q").value.trim();
  const slot = root.querySelector("#catalog-results");
  slot.innerHTML = `<div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div>`;
  const res = await api.searchCatalog(ws, q);
  if (!res.ok) {
    slot.innerHTML = errorState({ ...res, error: `Catalog search failed: ${apiStatusLabel(res)}` });
    return;
  }
  const items = Array.isArray(res.data) ? res.data : [];
  if (!items.length) {
    slot.innerHTML = emptyState(`No capabilities matched ${q ? `q=${q}` : "(empty filter)"} in this workspace.`, { icon: "icon-search" });
    return;
  }
  const table = dataTable({
    columns: [
      { key: "name", label: "Name", render: (c) => `<a href="#/capabilities/${escape(c.id)}" class="font-bold hover:underline">${escape(c.name)}</a><span class="block text-[.62rem] text-muted mono">${escape(c.id)}</span>` },
      { key: "description", label: "Description", render: (c) => `<span class="text-muted">${escape(c.description || "—")}</span>` },
      { key: "created", label: "Created", render: (c) => `<span class="mono text-muted">${escape(formatRelative(c.created_at))}</span>` },
      { key: "selfEvolve", label: "Self-evolve", render: (c) => statusPill(c.self_evolve?.enabled ? "on" : "off", c.self_evolve?.enabled ? "warn" : "neutral") },
    ],
    rows: items,
    emptyMessage: "No capabilities matched.",
    emptyIcon: "icon-search",
  });
  slot.innerHTML = `${table}<p class="mt-3 text-[.62rem] text-muted">${items.length} match${items.length === 1 ? "" : "es"}.</p>`;
}
