// User detail page. Reached via /users/{id}. Surfaces the
// single-user GET endpoint plus role / email metadata.
import * as api from "../api.js";
import { escape, formatRelative, apiStatusLabel } from "../utils.js";

export async function renderUserDetail(route) {
  const root = window.document.getElementById("view");
  const id = route?.params?.id;
  if (!root) return "";
  if (!id) {
    root.innerHTML = `<p class="panel p-5 mt-5 text-[.78rem] text-muted">No user id in the URL.</p>`;
    return;
  }
  root.innerHTML = `<section class="panel p-5"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
  const res = await api.getUser(id);
  if (!res.ok) {
    root.innerHTML = `<p class="panel p-5 mt-5 text-[.78rem] text-muted">Could not load user: ${escape(apiStatusLabel(res))}</p>`;
    return;
  }
  const u = res.data || {};
  const se = u.self_evolve || {};
  root.innerHTML = `<section class="mt-5 space-y-5">
    <article class="panel p-5 sm:p-6">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <div class="eyebrow flex items-center gap-2">
            <a href="#/operations/users" class="hover:text-ink">Users</a>
            <span class="text-[#b2b3af]">/</span>
            <span>User</span>
          </div>
          <h1 class="mt-2 text-[1.4rem] font-bold tracking-[-.04em]">${escape(u.name || u.email || u.id)}</h1>
          <p class="mt-1 text-[.7rem] text-muted mono">${escape(u.id)}</p>
        </div>
        <span class="status-pill ${u.role === "admin" ? "danger" : (u.role === "writer" ? "warn" : "neutral")} !px-2 !py-1">${escape(u.role || "—")}</span>
      </div>
      <dl class="mt-5 grid gap-3 sm:grid-cols-3">
        <div><dt class="eyebrow">Email</dt><dd class="mt-1 text-[.78rem] font-bold mono">${escape(u.email || "—")}</dd></div>
        <div><dt class="eyebrow">Created</dt><dd class="mt-1 text-[.78rem] font-bold">${escape(formatRelative(u.created_at))}</dd></div>
        <div><dt class="eyebrow">Updated</dt><dd class="mt-1 text-[.78rem] font-bold">${escape(formatRelative(u.updated_at))}</dd></div>
      </dl>
    </article>
    <article class="panel p-5 sm:p-6">
      <div class="eyebrow">Self-evolve config</div>
      <p class="mt-2 text-[.66rem] text-muted">${se.enabled ? "Self-evolution is enabled for this user." : "Self-evolution is not enabled."}</p>
      ${se.enabled ? `<dl class="mt-3 grid gap-3 sm:grid-cols-2 text-[.7rem]">
        <div><dt class="eyebrow">Min score</dt><dd class="mt-1 mono">${escape(String(se.min_score ?? "—"))}</dd></div>
        <div><dt class="eyebrow">Max revisions</dt><dd class="mt-1 mono">${escape(String(se.max_revisions ?? "—"))}</dd></div>
      </dl>` : ""}
    </article>
  </section>`;
}