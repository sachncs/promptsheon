// User detail page. Reached via /users/{id}. Surfaces the single-user
// GET endpoint plus role / email metadata.

import * as api from "../api.js";
import { escape, formatRelative, apiStatusLabel } from "../utils.js";
import { statusPill, pageHeader, panel, errorState } from "../ui.js";

const ROLE_TONES = { admin: "danger", writer: "warn", reader: "neutral" };
function roleTone(r) { return ROLE_TONES[r] || "neutral"; }

export async function renderUserDetail(route) {
  const root = window.document.getElementById("view");
  const id = route?.params?.id;
  if (!root) return "";
  if (!id) {
    root.innerHTML = `<div class="mt-5">${errorState({ ok: false, error: "No user id in the URL." })}</div>`;
    return;
  }
  root.innerHTML = `<section class="panel p-5"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;
  const res = await api.getUser(id);
  if (!res.ok) {
    root.innerHTML = `<div class="mt-5">${errorState({ ...res, error: `Could not load user: ${apiStatusLabel(res)}` })}</div>`;
    return;
  }
  const u = res.data || {};
  const se = u.self_evolve || {};
  const breadcrumbs = `<nav class="eyebrow flex items-center gap-2" aria-label="Breadcrumb">
    <a href="#/operations/users" class="hover:text-ink">Users</a>
    <span class="text-[#b2b3af]">/</span>
    <span>User</span>
  </nav>`;
  root.innerHTML = `${pageHeader({
    eyebrow: breadcrumbs,
    title: u.name || u.email || u.id,
    description: u.id,
    actions: statusPill(u.role || "—", roleTone(u.role)),
  })}
  <section class="mt-5 space-y-5">
    ${panel({
      eyebrow: "User metadata",
      title: "Identity",
      body: `
        <dl class="grid gap-3 sm:grid-cols-3">
          <div><dt class="eyebrow">Email</dt><dd class="mt-1 text-[.78rem] font-bold mono">${escape(u.email || "—")}</dd></div>
          <div><dt class="eyebrow">Created</dt><dd class="mt-1 text-[.78rem] font-bold">${escape(formatRelative(u.created_at))}</dd></div>
          <div><dt class="eyebrow">Updated</dt><dd class="mt-1 text-[.78rem] font-bold">${escape(formatRelative(u.updated_at))}</dd></div>
        </dl>
      `,
    })}
    ${panel({
      eyebrow: "Self-evolve config",
      title: se.enabled ? "Enabled" : "Disabled",
      body: `
        <p class="mt-2 text-[.66rem] text-muted">${se.enabled ? "Self-evolution is enabled for this user." : "Self-evolution is not enabled."}</p>
        ${se.enabled ? `<dl class="mt-3 grid gap-3 sm:grid-cols-2 text-[.7rem]">
          <div><dt class="eyebrow">Min score</dt><dd class="mt-1 mono">${escape(String(se.min_score ?? "—"))}</dd></div>
          <div><dt class="eyebrow">Max revisions</dt><dd class="mt-1 mono">${escape(String(se.max_revisions ?? "—"))}</dd></div>
        </dl>` : ""}
      `,
    })}
  </section>`;
}
