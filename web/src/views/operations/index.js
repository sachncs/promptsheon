import * as api from "../../api.js";
import { escape, formatRelative, apiStatusLabel } from "../../utils.js";

function pill(text, tone = "neutral") {
  return `<span class="status-pill ${tone} !px-2 !py-1"><span class="status-dot"></span>${escape(text)}</span>`;
}

const TABS = [
  { key: "alerts", label: "Alerts", href: "#/operations/alerts" },
  { key: "webhooks", label: "Webhooks", href: "#/operations/webhooks" },
  { key: "vault", label: "Vault", href: "#/operations/vault" },
  { key: "providers", label: "Providers", href: "#/operations/providers" },
  { key: "users", label: "Users", href: "#/operations/users" },
  { key: "reasoning", label: "Reasoning", href: "#/operations/reasoning" }
];

export function tabNav(active) {
  return `<nav class="flex flex-wrap items-center gap-1 rounded-xl bg-paper p-1" aria-label="Operations navigation">${TABS.map((t) => {
    const on = t.key === active;
    return `<a href="${t.href}" class="rounded-md ${on ? "bg-ink text-paper" : "text-muted hover:text-ink"} px-3 py-1.5 text-[.66rem] font-${on ? "bold" : "semibold"}">${escape(t.label)}</a>`;
  }).join("")}</nav>`;
}

function errorPanel(text) {
  return `<p class="panel p-5 mt-5 text-[.78rem] text-muted">${escape(text)}</p>`;
}

async function alertsTab() {
  const [rulesRes, groupsRes, activeRes] = await Promise.all([
    api.listAlertRules().catch((e) => ({ ok: false, error: String(e?.message || e) })),
    api.listNotificationGroups().catch((e) => ({ ok: false, error: String(e?.message || e) })),
    api.listAlerts().catch((e) => ({ ok: false, error: String(e?.message || e) }))
  ]);
  if (!rulesRes.ok) return errorPanel(`Alert rules unavailable${rulesRes.error ? ` (${escape(rulesRes.error)})` : ""}.`);
  if (!groupsRes.ok) return errorPanel(`Notification groups unavailable${groupsRes.error ? ` (${escape(groupsRes.error)})` : ""}.`);
  const rules = rulesRes.data || [];
  const groups = groupsRes.data || [];
  const active = (activeRes.data || []).filter((a) => a.status === "active" || a.status === "pending");
  return `<section class="mt-5 grid gap-5 xl:grid-cols-[minmax(0,2fr)_minmax(280px,.85fr)]">
    <article class="panel p-5 sm:p-6">
      <div class="flex items-center justify-between"><div><div class="eyebrow">Alert rules</div><h2 class="mt-1 text-[1rem] font-bold">${rules.length} configured</h2></div><button id="alert-new" class="quiet-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>New rule</button></div>
      ${rules.length ? `<table class="mt-4 w-full text-[.7rem]"><thead><tr class="text-left text-[.6rem] uppercase tracking-wider text-muted"><th class="py-1 font-bold">Name</th><th class="py-1 font-bold">Type</th><th class="py-1 font-bold">Severity</th><th class="py-1 font-bold">Threshold</th><th class="py-1 font-bold">Duration</th><th class="py-1 font-bold">Window</th><th class="py-1 font-bold text-right"></th></tr></thead><tbody>${rules.map((rule) => `<tr class="border-t border-line/60" data-rule-id="${escape(rule.id)}">
        <td class="py-2"><span class="font-bold">${escape(rule.name || "—")}</span><span class="block text-[.62rem] text-muted">${escape(rule.id)}</span></td>
        <td class="py-2 mono text-[.66rem]">${escape(rule.type || "—")}</td>
        <td class="py-2">${pill(rule.severity || "—", ({ critical: "danger", high: "danger", medium: "warn", low: "neutral" })[rule.severity] || "neutral")}</td>
        <td class="py-2 mono">${escape(rule.threshold ?? "—")}</td>
        <td class="py-2 mono">${escape((rule.duration_minutes ?? rule.duration) ?? "—")}m</td>
        <td class="py-2 mono">${escape((rule.window_minutes ?? rule.window) ?? "—")}m</td>
        <td class="py-2 text-right"><button data-rule-delete="${escape(rule.id)}" class="rounded-md bg-rose-50 px-2 py-1 text-[.62rem] font-bold text-rose-700 hover:bg-rose-100">Delete</button></td>
      </tr>`).join("")}</tbody></table>` : `<p class="mt-3 text-[.7rem] text-muted">No alert rules. Create one to start receiving signals.</p>`}
    </article>
    <article class="panel p-5 sm:p-6">
      <div class="flex items-center justify-between"><div><div class="eyebrow">Notification groups</div><h2 class="mt-1 text-[1rem] font-bold">${groups.length} configured</h2></div><button id="group-new" class="quiet-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>New group</button></div>
      ${groups.length ? `<ul class="mt-3 space-y-2 text-[.7rem]">${groups.map((g) => `<li class="rounded-xl bg-paper p-3"><span class="font-bold">${escape(g.name || "—")}</span><span class="ml-2 text-[.62rem] text-muted mono">${escape((g.channels || []).join(" · ") || "no channels")}</span></li>`).join("")}</ul>` : `<p class="mt-3 text-[.7rem] text-muted">No notification groups.</p>`}
      <div class="mt-5"><div class="eyebrow">Signals firing now</div>${active.length ? `<ul class="mt-2 space-y-1 text-[.7rem]"><li class="rounded-md bg-rose-50 px-2 py-1 text-rose-800">${active.length} active</li></ul>` : `<p class="mt-1 text-[.66rem] text-muted">All clear.</p>`}</div>
    </article>
  </section>`;
}

async function webhooksTab() {
  const res = await api.listWebhooks();
  if (!res.ok) return errorPanel(`Webhooks unavailable${res.error ? ` (${escape(res.error)})` : ""}.`);
  const list = res.data || [];
  return `<section class="mt-5">
    <article class="panel p-5 sm:p-6">
      <div class="flex items-center justify-between"><div><div class="eyebrow">Webhook registry</div><h2 class="mt-1 text-[1rem] font-bold">${list.length} configured</h2></div><button id="webhook-new" class="quiet-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>Add webhook</button></div>
      ${list.length ? `<table class="mt-4 w-full text-[.7rem]"><thead><tr class="text-left text-[.6rem] uppercase tracking-wider text-muted"><th class="py-1 font-bold">URL</th><th class="py-1 font-bold">Events</th><th class="py-1 font-bold">Created</th><th class="py-1 font-bold text-right"></th></tr></thead><tbody>${list.map((w) => `<tr class="border-t border-line/60" data-webhook-id="${escape(w.id)}">
        <td class="py-2 mono truncate max-w-[20rem]">${escape(w.url)}</td>
        <td class="py-2">${escape((w.events || []).join(", ") || "—")}</td>
        <td class="py-2 text-muted">${escape(formatRelative(w.created_at))}</td>
        <td class="py-2 text-right"><button data-webhook-delete="${escape(w.id)}" class="rounded-md bg-rose-50 px-2 py-1 text-[.62rem] font-bold text-rose-700 hover:bg-rose-100">Delete</button></td>
      </tr>`).join("")}</tbody></table>` : `<p class="mt-3 text-[.7rem] text-muted">No webhooks registered.</p>`}
    </article>
  </section>`;
}

async function vaultTab() {
  const res = await api.listVaultKeys();
  if (!res.ok) return errorPanel(`Vault unavailable${res.error ? ` (${escape(res.error)})` : ""}.`);
  const list = res.data || [];
  return `<section class="mt-5">
    <article class="panel p-5 sm:p-6">
      <div class="flex items-center justify-between"><div><div class="eyebrow">Vault keys</div><h2 class="mt-1 text-[1rem] font-bold">${list.length} stored</h2><p class="mt-1 text-[.7rem] text-muted">Key material never leaves the daemon. You can only list, save, or delete.</p></div><button id="vault-new" class="quiet-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>Save key</button></div>
      ${list.length ? `<table class="mt-4 w-full text-[.7rem]"><thead><tr class="text-left text-[.6rem] uppercase tracking-wider text-muted"><th class="py-1 font-bold">Provider</th><th class="py-1 font-bold">Key name</th><th class="py-1 font-bold">Created</th><th class="py-1 font-bold text-right"></th></tr></thead><tbody>${list.map((k) => `<tr class="border-t border-line/60" data-vault-id="${escape(k.id)}">
        <td class="py-2 mono">${escape(k.provider_name || "—")}</td>
        <td class="py-2">${escape(k.name || k.key_name || "—")}</td>
        <td class="py-2 text-muted">${escape(formatRelative(k.created_at))}</td>
        <td class="py-2 text-right"><button data-vault-delete="${escape(k.id)}" class="rounded-md bg-rose-50 px-2 py-1 text-[.62rem] font-bold text-rose-700 hover:bg-rose-100">Delete</button></td>
      </tr>`).join("")}</tbody></table>` : `<p class="mt-3 text-[.7rem] text-muted">No keys stored.</p>`}
    </article>
  </section>`;
}

async function providersTab() {
  const res = await api.listProviders();
  if (!res.ok) return errorPanel(`Providers unavailable${res.error ? ` (${escape(res.error)})` : ""}.`);
  const list = res.data?.providers || [];
  return `<section class="mt-5">
    <article class="panel p-5 sm:p-6">
      <div class="eyebrow">Registered providers</div>
      <div class="mt-3 space-y-2">${list.map((p) => `<div class="flex items-center justify-between rounded-xl bg-paper p-3"><span class="text-[.78rem] font-bold">${escape(p)}</span><button data-provider-test="${escape(p)}" class="quiet-button !text-[.66rem]"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-play"/></svg>Test connection</button></div>`).join("")}</div>
      <p id="provider-test-result" class="mt-3 text-[.68rem] text-muted">Click a provider to run a connectivity smoke test.</p>
    </article>
  </section>`;
}

async function usersTab() {
  const res = await api.listUsers();
  if (!res.ok) {
    const reason = res.status === 403 ? "Forbidden — admin role required to manage users." : apiStatusLabel(res);
    return errorPanel(`Users unavailable: ${escape(reason)}`);
  }
  const list = res.data || [];
  return `<section class="mt-5">
    <article class="panel p-5 sm:p-6">
      <div class="flex items-center justify-between"><div><div class="eyebrow">Users</div><h2 class="mt-1 text-[1rem] font-bold">${list.length} total</h2></div><button id="user-new" class="quiet-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>Create user</button></div>
      ${list.length ? `<table class="mt-4 w-full text-[.7rem]"><thead><tr class="text-left text-[.6rem] uppercase tracking-wider text-muted"><th class="py-1 font-bold">Name</th><th class="py-1 font-bold">Email</th><th class="py-1 font-bold">Role</th><th class="py-1 font-bold text-right"></th></tr></thead><tbody>${list.map((u) => `<tr class="border-t border-line/60" data-user-id="${escape(u.id)}">
        <td class="py-2"><span class="font-bold">${escape(u.name || u.id)}</span><span class="block text-[.62rem] text-muted mono">${escape(u.id)}</span></td>
        <td class="py-2 mono text-[.66rem]">${escape(u.email || "—")}</td>
        <td class="py-2"><span class="status-pill ${u.role === "admin" ? "good" : u.role === "writer" ? "warn" : "neutral"} !px-2 !py-1">${escape(u.role || "—")}</span></td>
        <td class="py-2 text-right"><button data-user-delete="${escape(u.id)}" class="rounded-md bg-rose-50 px-2 py-1 text-[.62rem] font-bold text-rose-700 hover:bg-rose-100">Delete</button></td>
      </tr>`).join("")}</tbody></table>` : `<p class="mt-3 text-[.7rem] text-muted">No users.</p>`}
    </article>
  </section>`;
}

async function reasoningTab() {
  return `<section class="mt-5">
    <article class="panel p-5 sm:p-6">
      <div class="eyebrow">Reasoning compiler</div>
      <p class="mt-1 text-[.7rem] text-muted">Paste an intent; the compiler returns the CapabilityPlan it would execute. Use to validate routing against the live catalog.</p>
      <form id="reasoning-form" class="mt-4 space-y-3">
        <div><label class="eyebrow mb-2 block" for="rz-intent">Intent</label><textarea id="rz-intent" class="field min-h-28 resize-y" required data-autofocus placeholder="e.g. Summarize today's GitHub issues for the dashboard project and email the digest to the on-call."></textarea></div>
        <div><label class="eyebrow mb-2 block" for="rz-ws">Workspace (optional)</label><input id="rz-ws" class="field mono" placeholder="workspace id (optional)" /></div>
        <p id="rz-error" class="hidden rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800"></p>
        <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
          <button type="submit" class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-spark"/></svg>Compile</button>
        </div>
      </form>
      <div id="rz-result" class="mt-5"></div>
    </article>
  </section>`;
}

export async function renderOperations(route) {
  const root = window.document.getElementById("view");
  if (!root) return "";
  const tab = route?.params?.tab || (route?.path === "/operations" ? "alerts" : "alerts");

  root.innerHTML = `<section class="panel p-6"><div class="skeleton h-3 w-32"></div><div class="skeleton mt-4 h-12 w-full"></div></section>`;

  const renderers = { alerts: alertsTab, webhooks: webhooksTab, vault: vaultTab, providers: providersTab, users: usersTab, reasoning: reasoningTab };
  const renderer = renderers[tab] || alertsTab;
  const body = await renderer();

  const shell = `
    <section>
      <div class="eyebrow">Operator surface</div>
      <h1 class="mt-2 text-[1.4rem] font-bold tracking-[-.04em]">${escape(tab[0].toUpperCase() + tab.slice(1))}</h1>
      <div class="mt-4">${tabNav(tab)}</div>
    </section>
    ${body}
  `;
  root.innerHTML = shell;
  attach(tab, root);
  return shell;
}

function attach(tab, root) {
  if (tab === "alerts") {
    root.querySelector("#alert-new")?.addEventListener("click", async () => {
      const { openNewRuleModal } = await import("./modals.js");
      await openNewRuleModal(root, () => window.location.reload());
    });
    root.querySelectorAll("[data-rule-delete]").forEach((b) => b.addEventListener("click", async () => {
      if (!window.confirm("Delete this rule?")) return;
      const id = b.dataset.ruleDelete;
      const result = await api.deleteAlertRule(id);
      if (!result.ok) {
        window.alert(`Delete failed: ${apiStatusLabel(result)}`);
        return;
      }
      window.location.reload();
    }));
    root.querySelector("#group-new")?.addEventListener("click", async () => {
      const { openNewGroupModal } = await import("./modals.js");
      await openNewGroupModal(root, () => window.location.reload());
    });
  } else if (tab === "webhooks") {
    root.querySelector("#webhook-new")?.addEventListener("click", async () => {
      const { openNewWebhookModal } = await import("./modals.js");
      await openNewWebhookModal(root, () => window.location.reload());
    });
    root.querySelectorAll("[data-webhook-delete]").forEach((b) => b.addEventListener("click", async () => {
      if (!window.confirm("Delete this webhook?")) return;
      const result = await api.deleteWebhook(b.dataset.webhookDelete);
      if (!result.ok) { window.alert(apiStatusLabel(result)); return; }
      window.location.reload();
    }));
  } else if (tab === "vault") {
    root.querySelector("#vault-new")?.addEventListener("click", async () => {
      const { openVaultKeyModal } = await import("./modals.js");
      await openVaultKeyModal(root, () => window.location.reload());
    });
    root.querySelectorAll("[data-vault-delete]").forEach((b) => b.addEventListener("click", async () => {
      if (!window.confirm("Delete this key?")) return;
      const result = await api.deleteVaultKey(b.dataset.vaultDelete);
      if (!result.ok) { window.alert(apiStatusLabel(result)); return; }
      window.location.reload();
    }));
  } else if (tab === "providers") {
    root.querySelectorAll("[data-provider-test]").forEach((b) => b.addEventListener("click", async () => {
      const name = b.dataset.providerTest;
      const model = window.prompt(`Model to test with ${name}?`, "gpt-4o-mini");
      if (!model) return;
      const slot = root.querySelector("#provider-test-result");
      slot.textContent = `Testing ${name}…`;
      const result = await api.testProvider(name, model);
      if (!result.ok) {
        slot.textContent = `${name} test failed: ${apiStatusLabel(result)}`;
        return;
      }
      const content = result.data?.content ? ` — "${result.data.content.replace(/[\\n\\r]/g, " ").slice(0, 80)}…"` : "";
      slot.textContent = `${name}: ${result.data?.status || "ok"} (${result.data?.latency_ms || 0}ms)${content}`;
    }));
  } else if (tab === "users") {
    root.querySelector("#user-new")?.addEventListener("click", async () => {
      const { openNewUserModal } = await import("./modals.js");
      await openNewUserModal(root, () => window.location.reload());
    });
    root.querySelectorAll("[data-user-delete]").forEach((b) => b.addEventListener("click", async () => {
      if (!window.confirm("Delete this user?")) return;
      const result = await api.deleteUser(b.dataset.userDelete);
      if (!result.ok) { window.alert(apiStatusLabel(result)); return; }
      window.location.reload();
    }));
  } else if (tab === "reasoning") {
    root.querySelector("#reasoning-form")?.addEventListener("submit", async (event) => {
      event.preventDefault();
      const error = root.querySelector("#rz-error");
      const slot = root.querySelector("#rz-result");
      error.classList.add("hidden");
      slot.innerHTML = `<div class="skeleton h-12 w-full"></div>`;
      const intent = root.querySelector("#rz-intent").value.trim();
      const workspace_id = root.querySelector("#rz-ws").value.trim();
      const result = await api.compileReasoning(intent, workspace_id);
      if (!result.ok) {
        slot.innerHTML = "";
        error.textContent = apiStatusLabel(result);
        error.classList.remove("hidden");
        return;
      }
      slot.innerHTML = `<div class="rounded-xl bg-paper p-4"><div class="eyebrow">Plan</div><pre class="mt-2 overflow-x-auto text-[.66rem] mono">${escape(JSON.stringify(result.data, null, 2))}</pre></div>`;
    });
  }
}
