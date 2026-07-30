// Version detail page. Surfaces GET /api/v1/versions/{id}
// for the single-version view. Reached by clicking a version
// row in the capability-detail page.
import * as api from "../api.js";
import { escape, formatRelative, apiStatusLabel } from "../utils.js";

export async function renderVersionDetail(route) {
  const root = window.document.getElementById("view");
  const id = route?.params?.id;
  if (!root) return "";
  if (!id) {
    root.innerHTML = `<p class="panel p-5 mt-5 text-[.78rem] text-muted">No version id in the URL.</p>`;
    return;
  }
  root.innerHTML = `<section class="panel p-5"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
  const vRes = await api.getVersion(id);
  if (!vRes.ok) {
    root.innerHTML = `<p class="panel p-5 mt-5 text-[.78rem] text-muted">Could not load version: ${escape(apiStatusLabel(vRes))}</p>`;
    return;
  }
  const v = vRes.data;
  root.innerHTML = `<section class="mt-5 space-y-5">
    <article class="panel p-5 sm:p-6">
      <div class="eyebrow flex items-center gap-2">
        <a href="#/capabilities/${escape(v.capability_id)}" class="hover:text-ink">Capability</a>
        <span class="text-[#b2b3af]">/</span>
        <span>Version v${escape(String(v.version))}</span>
      </div>
      <h1 class="mt-2 text-[1.5rem] font-bold tracking-[-.04em]">v${escape(String(v.version))}</h1>
      <p class="mt-1 text-[.7rem] text-muted mono">${escape(v.id)}</p>
      <dl class="mt-5 grid gap-3 sm:grid-cols-3">
        <div><dt class="eyebrow">Manifest hash</dt><dd class="mt-1 break-all text-[.7rem] mono">${escape(v.manifest_hash || "—")}</dd></div>
        <div><dt class="eyebrow">Created by</dt><dd class="mt-1 text-[.78rem] font-bold">${escape(v.created_by || "—")}</dd></div>
        <div><dt class="eyebrow">Created</dt><dd class="mt-1 text-[.78rem] font-bold">${escape(formatRelative(v.created_at))}</dd></div>
      </dl>
    </article>
    <article class="panel p-5 sm:p-6">
      <div class="eyebrow">Manifest</div>
      <pre class="mt-3 overflow-x-auto rounded-lg bg-paper p-4 text-[.66rem] mono">${escape(JSON.stringify(v.manifest, null, 2))}</pre>
    </article>
  </section>`;
}