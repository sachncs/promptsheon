// Catalog view. Cross-workspace capability search via
// GET /api/v1/catalog/capabilities. Reached via
// /operations/catalog. Operator picks a workspace + query
// string and gets a flat list of matching capabilities with a
// link into the capability-detail page.
import * as api from "../api.js";
import { escape, formatRelative, apiStatusLabel } from "../utils.js";
import { go } from "../router.js";

const SETTINGS_TABS = [
  { key: "catalog", label: "Catalog", href: "#/operations/catalog" }
];

export function catalogTab() {
  return SETTINGS_TABS[0].label;
}

export function isCatalogTab(tab) {
  return tab === "catalog";
}

export async function renderCatalogTab(root) {
  if (!root) return "";
  root.innerHTML = `<section class="panel p-5"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
  const wsRes = await api.listWorkspaces();
  if (!wsRes.ok) {
    root.innerHTML = `<p class="panel p-5 mt-5 text-[.78rem] text-muted">Workspaces unavailable: ${escape(apiStatusLabel(wsRes))}</p>`;
    return;
  }
  const workspaces = wsRes.data || [];
  root.innerHTML = `<section class="mt-5 space-y-5">
    <article class="panel p-5 sm:p-6">
      <div class="flex flex-wrap items-end justify-between gap-3">
        <div>
          <div class="eyebrow">Cross-workspace catalog</div>
          <h1 class="mt-2 text-[1.4rem] font-bold tracking-[-.04em]">Search</h1>
          <p class="mt-1 text-[.78rem] text-muted">Catalog search spans every workspace. The dropdown picks the workspace; the input filters by name substring.</p>
        </div>
      </div>
      <form id="catalog-form" class="mt-4 grid gap-3 sm:grid-cols-[minmax(220px,1fr)_1fr_auto]">
        <select id="catalog-ws" name="workspace_id" class="field">
          ${workspaces.map((w, i) => `<option value="${escape(w.id)}" ${i === 0 ? "selected" : ""}>${escape(w.name)}</option>`).join("")}
        </select>
        <input id="catalog-q" name="q" class="field" placeholder="substring filter (optional)" />
        <button type="submit" class="primary-button">Search</button>
      </form>
      <div id="catalog-results" class="mt-5"></div>
    </article>
  </section>`;
  attach(root);
  // Trigger an initial search on the first workspace.
  if (workspaces.length) {
    root.querySelector("#catalog-ws").value = workspaces[0].id;
    await runCatalogSearch(root);
  }
}

function attach(root) {
  root.querySelector("#catalog-form")?.addEventListener("submit", (event) => {
    event.preventDefault();
    runCatalogSearch(root);
  });
}

async function runCatalogSearch(root) {
  const ws = root.querySelector("#catalog-ws").value;
  const q = root.querySelector("#catalog-q").value.trim();
  const slot = root.querySelector("#catalog-results");
  slot.innerHTML = `<div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div>`;
  const res = await api.searchCatalog(ws, q);
  if (!res.ok) {
    slot.innerHTML = `<p class="panel p-5 text-[.78rem] text-muted">Catalog search failed: ${escape(apiStatusLabel(res))}</p>`;
    return;
  }
  const items = Array.isArray(res.data) ? res.data : [];
  if (!items.length) {
    slot.innerHTML = `<p class="text-[.7rem] text-muted">No capabilities matched ${q ? `q=${escape(q)}` : "(empty filter)"} in this workspace.</p>`;
    return;
  }
  slot.innerHTML = `<table class="mt-2 w-full text-[.7rem]"><thead><tr class="text-left text-[.6rem] uppercase tracking-wider text-muted"><th class="py-1 font-bold">Name</th><th class="py-1 font-bold">Description</th><th class="py-1 font-bold">Created</th><th class="py-1 font-bold">Self-evolve</th></tr></thead><tbody>${items.map((c) => `<tr class="border-t border-line/60">
    <td class="py-2"><a href="#/capabilities/${escape(c.id)}" class="font-bold hover:underline">${escape(c.name)}</a><span class="block text-[.62rem] text-muted mono">${escape(c.id)}</span></td>
    <td class="py-2 text-[.66rem] text-muted">${escape(c.description || "—")}</td>
    <td class="py-2 mono">${escape(formatRelative(c.created_at))}</td>
    <td class="py-2">${c.self_evolve?.enabled ? `<span class="status-pill warn !px-2 !py-1">on</span>` : `<span class="status-pill neutral !px-2 !py-1">off</span>`}</td>
  </tr>`).join("")}</tbody></table>
  <p class="mt-3 text-[.62rem] text-muted">${items.length} match${items.length === 1 ? "" : "es"}.</p>`;
}