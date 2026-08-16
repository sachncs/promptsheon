// Version detail page. Surfaces GET /api/v1/versions/{id} for the
// single-version view. Reached by clicking a version row in the
// capability-detail page.

import * as api from "../api.js";
import { escape, formatRelative, apiStatusLabel } from "../utils.js";
import { pageHeader, panel, errorState } from "../ui.js";

export async function renderVersionDetail(route) {
  const root = window.document.getElementById("view");
  const id = route?.params?.id;
  if (!root) return "";
  if (!id) {
    root.innerHTML = `<div class="mt-5">${errorState({ ok: false, error: "No version id in the URL." })}</div>`;
    return;
  }
  root.innerHTML = `<section class="panel p-5"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
  const vRes = await api.getVersion(id);
  if (!vRes.ok) {
    root.innerHTML = `<div class="mt-5">${errorState({ ...vRes, error: `Could not load version: ${apiStatusLabel(vRes)}` })}</div>`;
    return;
  }
  const v = vRes.data;
  const breadcrumbs = `<nav class="eyebrow flex items-center gap-2" aria-label="Breadcrumb">
    <a href="#/capabilities/${escape(v.capability_id)}" class="hover:text-ink">Capability</a>
    <span class="text-[#b2b3af]">/</span>
    <span>Version v${escape(String(v.version))}</span>
  </nav>`;
  root.innerHTML = `${pageHeader({
    eyebrow: breadcrumbs,
    title: `v${v.version}`,
    description: v.id,
  })}
  <section class="mt-5 space-y-5">
    ${panel({
      eyebrow: "Version metadata",
      title: `v${v.version}`,
      body: `
        <dl class="grid gap-3 sm:grid-cols-3">
          <div><dt class="eyebrow">Manifest hash</dt><dd class="mt-1 break-all text-[.7rem] mono">${escape(v.manifest_hash || "—")}</dd></div>
          <div><dt class="eyebrow">Created by</dt><dd class="mt-1 text-[.78rem] font-bold">${escape(v.created_by || "—")}</dd></div>
          <div><dt class="eyebrow">Created</dt><dd class="mt-1 text-[.78rem] font-bold">${escape(formatRelative(v.created_at))}</dd></div>
        </dl>
      `,
    })}
    ${panel({
      eyebrow: "Manifest",
      title: "Content",
      body: `<pre class="overflow-x-auto rounded-lg bg-paper p-4 text-[.66rem] mono">${escape(JSON.stringify(v.manifest, null, 2))}</pre>`,
    })}
  </section>`;
}
