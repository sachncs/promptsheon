// src/shell.js — top-level layout shell.
//
// Renders the sidebar, header, modal-root, and toast-root and wires
// the global keyboard handlers (mobile menu toggle, ESC, settings,
// notifications). The header pulls its organisation label from the
// connection settings module so a tenant can rename the workspace
// without editing HTML.

import { renderNav } from "./components/nav.js";
import { renderView } from "./views/index.js";
import { renderModalRoot } from "./components/modal-root.js";
import { currentRoute } from "./router.js";
import { escape } from "./utils.js";
import { renderConnectPrompt } from "./views/index.js";

export function renderAppShell() {
  attachModalHandlers();
  return `
    <div id="sidebar-shade" class="sidebar-shade fixed inset-0 z-30 bg-ink/40 lg:hidden"></div>
    ${renderNav()}
    <div class="min-h-screen lg:pl-[248px]">
      <header class="sticky top-0 z-20 border-b border-line/80 bg-paper/85 backdrop-blur-xl">
        <div class="mx-auto flex h-[70px] max-w-[1600px] items-center justify-between gap-4 px-5 sm:px-8 xl:px-10">
          <div class="flex min-w-0 items-center gap-3">
            <button id="mobile-menu" class="icon-button mobile-only shrink-0" aria-label="Open navigation">
              <svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-menu"/></svg>
            </button>
            <nav id="breadcrumbs" class="hidden items-center gap-2 text-[.72rem] text-muted sm:flex" aria-label="Breadcrumbs"></nav>
            <h2 id="page-title" class="truncate text-[.9rem] font-bold tracking-[-.025em]"></h2>
          </div>
          <div class="flex items-center gap-2.5">
            <label class="relative hidden w-52 md:block">
              <span class="sr-only">Search capabilities</span>
              <svg class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 fill-none stroke-[#9a9c99] stroke-[1.8]"><use href="#icon-search"/></svg>
              <input id="capability-search" class="field !h-9 !rounded-lg !border-transparent !bg-white/65 !py-1.5 !pl-9 !pr-12 !text-[.72rem]" placeholder="Search capabilities" aria-label="Search capabilities" />
              <kbd class="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 rounded border border-line bg-paper px-1.5 py-0.5 font-mono text-[.58rem] text-muted">⌘ K</kbd>
            </label>
            <button class="icon-button relative" aria-label="Notifications" data-open-notifications>
              <svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-bell"/></svg>
              <span class="absolute right-[5px] top-[5px] h-1.5 w-1.5 rounded-full bg-accent ring-2 ring-paper"></span>
            </button>
            <div class="ml-1 hidden h-7 w-px bg-line sm:block"></div>
            <button data-open-settings class="flex items-center gap-2 rounded-lg p-1 transition hover:bg-white/80" aria-label="Open connection settings">
              <span id="user-avatar" class="grid h-8 w-8 place-items-center rounded-lg bg-[#dce7bd] text-[.68rem] font-bold text-[#52632d]">—</span>
              <span class="hidden text-left sm:block">
                <span id="user-name" class="block text-[.73rem] font-bold">—</span>
                <span id="user-role" class="block text-[.62rem] text-muted">—</span>
              </span>
              <svg class="hidden h-3.5 w-3.5 fill-none stroke-[#898c8a] stroke-2 sm:block"><use href="#icon-chevron"/></svg>
            </button>
          </div>
        </div>
      </header>
      <main id="view" class="mx-auto max-w-[1600px] px-5 py-7 sm:px-8 sm:py-9 xl:px-10">
        <div id="connect-banner"></div>
        <div id="view-body"></div>
      </main>
    </div>
    ${renderModalRoot()}
  `;
}

// Mount the global shell — sidebar/header/etc — and start the
// first render.  All data-driven rendering (sidebar counts, runtime
// pill, header org label) happens here so the template stays static.
export async function mountShell() {
  const app = document.getElementById("app");
  if (!app) return;
  app.innerHTML = renderAppShell();
  bindMobileNav();
  await renderUserBadge();
  await renderRuntimeStatus();
  renderInitialState();
}

export function renderInitialState() {
  renderView(currentRoute());
  // First-run bootstrap: when the dashboard has no API key,
  // surface the setup modal so a fresh install can mint its
  // initial admin key via the UI instead of curl. The modal
  // handles its own X-Bootstrap-Token header.
  import("./views/first-run-setup.js").then(({ shouldOfferFirstRun, openFirstRunSetupModal }) => {
    if (!shouldOfferFirstRun()) return;
    const root = document.getElementById("modal-root");
    if (root && root.childElementCount === 0) {
      openFirstRunSetupModal(root, { onReady: () => window.location.reload() });
    }
  });
}

function bindMobileNav() {
  const button = document.getElementById("mobile-menu");
  const sidebar = document.getElementById("sidebar");
  const shade = document.getElementById("sidebar-shade");
  if (!button || !sidebar || !shade) return;

  function setOpen(open) {
    sidebar.classList.toggle("open", open);
    shade.classList.toggle("open", open);
    button.setAttribute("aria-expanded", String(open));
  }

  button.addEventListener("click", () => setOpen(!sidebar.classList.contains("open")));
  shade.addEventListener("click", () => setOpen(false));
}

async function renderUserBadge() {
  const avatar = document.getElementById("user-avatar");
  const name = document.getElementById("user-name");
  const role = document.getElementById("user-role");
  if (!avatar || !name || !role) return;

  const { loadSettings } = await import("./settings.js");
  const { ownerName } = await import("./state/owners.js");
  const settings = loadSettings();
  const identity = settings.user || null;
  const display = identity?.name || ownerName(identity?.id) || (settings.apiKey ? "Operator" : "Guest");
  const initials = display
    .split(/\s+/)
    .map((s) => s[0])
    .filter(Boolean)
    .slice(0, 2)
    .join("")
    .toUpperCase() || "PS";
  const roleText = identity?.role || (settings.apiKey ? "Administrator" : "Not connected");

  avatar.textContent = escape(initials);
  name.textContent = escape(display);
  role.textContent = escape(roleText);
}

async function renderRuntimeStatus() {
  const { loadSettings } = await import("./settings.js");
  const settings = loadSettings();
  const pillEl = document.getElementById("runtime-status-pill");
  const labelEl = document.querySelector("[data-runtime-label]");
  const versionEl = document.getElementById("runtime-version");
  const uptimeEl = document.getElementById("runtime-uptime");

  // Without an API key we never make these calls; render a neutral
  // "Not connected" pill and return.
  if (!pillEl || !settings.apiKey) {
    if (pillEl) {
      pillEl.classList.remove("warn", "good", "danger");
      pillEl.classList.add("neutral");
    }
    if (labelEl) labelEl.textContent = "Not connected";
    if (versionEl) versionEl.textContent = "—";
    if (uptimeEl) uptimeEl.textContent = "—";
    return;
  }

  const { getHealth, getReady } = await import("./api.js");
  const [health, ready] = await Promise.all([getHealth(), getReady()]);
  const ok = health?.ok && ready?.ok && ready.data?.status === "ready";

  if (pillEl) {
    pillEl.classList.remove("good", "warn", "danger", "neutral");
    pillEl.classList.add(ok ? "good" : "warn");
  }
  if (labelEl) {
    labelEl.textContent = ok
      ? "Healthy"
      : health?.ok
        ? ready?.data?.status || "Starting"
        : "Offline";
  }
  if (versionEl) versionEl.textContent = health?.ok ? (health.data?.version || "dev") : "—";
  if (uptimeEl) uptimeEl.textContent = health?.ok ? (health.data?.uptime || "—") : "—";
}

let modalHandlersAttached = false;
function attachModalHandlers() {
  if (modalHandlersAttached) return;
  modalHandlersAttached = true;
  document.addEventListener("click", (event) => {
    const closer = event.target.closest("[data-close-modal]");
    if (!closer) return;
    const root = document.getElementById("modal-root");
    if (root) root.replaceChildren();
  });
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape") {
      const root = document.getElementById("modal-root");
      if (root && root.childElementCount > 0) root.replaceChildren();
    }
  });
  document.addEventListener("click", async (event) => {
    const openSettings = event.target.closest("[data-open-settings]");
    if (openSettings) {
      event.preventDefault();
      const { openSettings: open } = await import("./components/settings-button.js");
      const root = document.getElementById("modal-root");
      if (root) await open(root);
      return;
    }
    const bell = event.target.closest("[data-open-notifications]");
    if (bell) {
      const { openNotificationsModal } = await import("./views/notifications-modal.js");
      const root = document.getElementById("modal-root");
      if (root) await openNotificationsModal(root);
    }
  });
}

// Re-export for callers (e.g. tests) that rely on the old surface.
export { renderConnectPrompt };
