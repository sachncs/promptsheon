import { escape } from "../utils.js";

export function renderNav() {
  return `
    <aside id="sidebar" class="sidebar fixed inset-y-0 left-0 z-40 flex w-[248px] flex-col overflow-y-auto bg-ink px-4 py-5 text-white lg:translate-x-0">
      <div class="flex items-center justify-between px-2">
        <a href="#/" class="flex items-center gap-2.5" aria-label="Promptsheon home">
          <span class="grid h-8 w-8 place-items-center rounded-[10px] bg-lime text-ink">
            <svg class="h-[18px] w-[18px] fill-none stroke-current stroke-[1.8]" viewBox="0 0 24 24"><path d="m12 2 8.7 5v10L12 22l-8.7-5V7L12 2Z"/><path d="m8 9 4-2 4 2v6l-4 2-4-2V9Z"/><path d="M12 7v10M8 9l8 6M16 9l-8 6"/></svg>
          </span>
          <span class="text-[.92rem] font-bold tracking-[-.03em]">promptsheon</span>
        </a>
      </div>
      <button class="mt-8 flex items-center gap-3 rounded-xl border border-white/10 bg-white/[.06] p-3 text-left transition hover:bg-white/[.1]" data-open-settings>
        <span class="grid h-8 w-8 place-items-center rounded-lg bg-[#303239] text-[.65rem] font-bold text-lime">AC</span>
        <span class="min-w-0 flex-1">
          <span class="block truncate text-[.75rem] font-bold">Acme Corporation</span>
          <span class="mt-0.5 block text-[.66rem] text-[#8f9299]">Connection settings</span>
        </span>
        <svg class="h-4 w-4 shrink-0 fill-none stroke-[#92959c] stroke-2"><use href="#icon-chevron"/></svg>
      </button>
      <div class="mt-8 px-2 eyebrow !text-[#6f727a]">Workspace</div>
      <nav class="mt-3 space-y-1" aria-label="Workspace navigation">
        <a class="nav-item" data-nav href="#/"><svg class="h-[17px] w-[17px] fill-none stroke-current stroke-[1.7]"><use href="#icon-grid"/></svg><span>Overview</span></a>
        <a class="nav-item" data-nav href="#/capabilities"><svg class="h-[17px] w-[17px] fill-none stroke-current stroke-[1.7]"><use href="#icon-layers"/></svg><span>Capabilities</span><span id="nav-capabilities-count" class="ml-auto text-[.67rem] text-[#777a82]">—</span></a>
        <a class="nav-item" data-nav href="#/releases"><svg class="h-[17px] w-[17px] fill-none stroke-current stroke-[1.7]"><use href="#icon-rocket"/></svg><span>Releases</span><span id="nav-releases-count" class="ml-auto text-[.67rem] text-[#777a82]">—</span></a>
        <a class="nav-item" data-nav href="#/audit"><svg class="h-[17px] w-[17px] fill-none stroke-current stroke-[1.7]"><use href="#icon-scroll"/></svg><span>Audit trail</span></a>
        <a class="nav-item" data-nav href="#/observability"><svg class="h-[17px] w-[17px] fill-none stroke-current stroke-[1.7]"><use href="#icon-pulse"/></svg><span>Observability</span></a>
        <a class="nav-item" data-nav href="#/evaluations"><svg class="h-[17px] w-[17px] fill-none stroke-current stroke-[1.7]"><use href="#icon-flask"/></svg><span>Evaluations</span></a>
        <a class="nav-item" data-nav href="#/guardrails"><svg class="h-[17px] w-[17px] fill-none stroke-current stroke-[1.7]"><use href="#icon-shield"/></svg><span>Guardrails</span></a>
        <a class="nav-item" data-nav href="#/logs"><svg class="h-[17px] w-[17px] fill-none stroke-current stroke-[1.7]"><use href="#icon-pulse"/></svg><span>Live logs</span></a>
      </nav>
      <div class="mt-8 px-2 eyebrow !text-[#6f727a]">Operations</div>
      <nav class="mt-3 space-y-1" aria-label="Operations navigation">
        <a class="nav-item" data-nav href="#/operations/alerts"><svg class="h-[17px] w-[17px] fill-none stroke-current stroke-[1.7]"><use href="#icon-warning"/></svg><span>Alerts</span></a>
        <a class="nav-item" data-nav href="#/operations/webhooks"><svg class="h-[17px] w-[17px] fill-none stroke-current stroke-[1.7]"><use href="#icon-code"/></svg><span>Webhooks</span></a>
        <a class="nav-item" data-nav href="#/operations/vault"><svg class="h-[17px] w-[17px] fill-none stroke-current stroke-[1.7]"><use href="#icon-database"/></svg><span>Vault</span></a>
        <a class="nav-item" data-nav href="#/operations/providers"><svg class="h-[17px] w-[17px] fill-none stroke-current stroke-[1.7]"><use href="#icon-play"/></svg><span>Providers</span></a>
        <a class="nav-item" data-nav href="#/operations/users" id="nav-operations-users"><svg class="h-[17px] w-[17px] fill-none stroke-current stroke-[1.7]"><use href="#icon-layers"/></svg><span>Users</span></a>
        <a class="nav-item" data-nav href="#/operations/reasoning"><svg class="h-[17px] w-[17px] fill-none stroke-current stroke-[1.7]"><use href="#icon-spark"/></svg><span>Reasoning</span></a>
      </nav>
      <div class="mt-auto pt-8">
        <div class="mb-3 rounded-xl border border-white/10 bg-white/[.045] p-3.5">
          <div class="flex items-center justify-between">
            <span class="eyebrow !text-[#858890]">Runtime status</span>
            <span id="runtime-status-pill" class="status-pill warn !bg-amber-400/10 !text-amber-400"><span class="status-dot"></span><span data-runtime-label>Connecting…</span></span>
          </div>
          <div class="mt-3 flex items-end justify-between">
            <span class="mono text-[.68rem] text-[#777a82]">build <span id="runtime-version">…</span></span>
            <span class="text-[.67rem] text-[#777a82]">uptime <span id="runtime-uptime">…</span></span>
          </div>
        </div>
        <button class="nav-item" data-open-settings><svg class="h-[17px] w-[17px] fill-none stroke-current stroke-[1.7]"><use href="#icon-settings"/></svg><span>Connection</span></button>
        <a class="nav-item" href="https://github.com/sachncs/promptsheon" target="_blank" rel="noopener"><svg class="h-[17px] w-[17px] fill-none stroke-current stroke-[1.7]"><use href="#icon-help"/></svg><span>Documentation</span><svg class="ml-auto h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-external"/></svg></a>
      </div>
    </aside>
  `;
}

export function renderConnectionBar() {
  return `<div id="connect-banner"></div>`;
}
