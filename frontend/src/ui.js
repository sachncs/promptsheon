// src/ui.js — shared UI primitives used by every view, modal, and component.
//
// The contract:
//   - Pure functions that return HTML strings; callers compose them.
//   - Every helper escapes user input via utils.escape; never paste
//     raw data into these templates.
//   - The strings use the design tokens declared in styles.css so a
//     single token change updates every view in one place.
//
// Why a string API instead of a component tree?  The dashboard renders
// directly into the DOM via innerHTML today. A virtual-DOM rewrite is
// out of scope; a shared template layer keeps the existing pipeline
// while removing the duplication that produced drift.

import { escape, apiStatusLabel } from "./utils.js";

// Tone names used across the dashboard. Keep this list small — every
// tone ships with a matching class in styles.css.
export const TONES = Object.freeze(["good", "warn", "danger", "info", "neutral"]);

function toneClass(tone) {
  return TONES.includes(tone) ? tone : "neutral";
}

// statusPill — single source for the colored tag used everywhere a
// value carries semantics (status, environment, severity, etc.).
export function statusPill(text, tone = "neutral") {
  return `<span class="status-pill ${toneClass(tone)}"><span class="status-dot"></span>${escape(text)}</span>`;
}

// pageHeader — title block + optional actions row. Every view page
// starts with this; using it everywhere guarantees the same eyebrow,
// title size, description max-width, and action alignment.
export function pageHeader({ eyebrow, title, description, actions = "" } = {}) {
  const eyebrowHtml = eyebrow
    ? `<div class="eyebrow">${escape(eyebrow)}</div>`
    : "";
  const titleHtml = title
    ? `<h1 class="page-title">${escape(title)}</h1>`
    : "";
  const descHtml = description
    ? `<p class="page-description">${escape(description)}</p>`
    : "";
  const actionsHtml = actions
    ? `<div class="page-actions">${actions}</div>`
    : "";

  if (!actionsHtml) {
    return `<header class="page-header"><div>${eyebrowHtml}${titleHtml}${descHtml}</div>${actionsHtml}</header>`;
  }
  return `<header class="page-header"><div class="min-w-0 flex-1">${eyebrowHtml}${titleHtml}${descHtml}</div>${actionsHtml}</header>`;
}

// panel — uniform rounded card used by every detail block. The
// optional eyebrow/title/rightSlot pattern lets views append a small
// toolbar without re-implementing the header.
export function panel({ eyebrow, title, rightSlot = "", body = "", className = "", padded = true } = {}) {
  const classes = ["panel"];
  if (padded) classes.push("p-5", "sm:p-6");
  if (className) classes.push(className);

  const header = eyebrow || title || rightSlot
    ? `<div class="flex flex-wrap items-end justify-between gap-3">
         <div class="min-w-0">
           ${eyebrow ? `<div class="eyebrow">${escape(eyebrow)}</div>` : ""}
           ${title ? `<h2 class="mt-1 text-[1rem] font-bold tracking-[-.025em]">${escape(title)}</h2>` : ""}
         </div>
         ${rightSlot ? `<div class="flex items-center gap-2">${rightSlot}</div>` : ""}
       </div>`
    : "";

  return `<article class="${classes.join(" ")}">${header}${header && body ? `<div class="mt-4">${body}</div>` : body}</article>`;
}

// emptyState — single render path for the "nothing here yet" copy.
// Accepts an optional icon-id from index.html's <symbol> set.
export function emptyState(message, { icon = "icon-grid", action = "" } = {}) {
  return `<div class="empty-state" role="status">
    <span class="empty-state-icon"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#${escape(icon)}"/></svg></span>
    <p>${escape(message)}</p>
    ${action ? `<div class="mt-3">${action}</div>` : ""}
  </div>`;
}

// errorState — used by every view that can fail to load. It
// understands the api.js result envelope so callers can just pass the
// raw response object.
export function errorState(result, { prefix = "" } = {}) {
  let message;
  let tone = "neutral";
  if (!result) {
    message = "Loading…";
  } else if (result.ok) {
    return "";
  } else if (result.status === 429) {
    message = "Rate limited — slowing down automatically.";
    tone = "warn";
  } else if (result.status === 401) {
    message = "An API key is required to view this section.";
    tone = "warn";
  } else {
    message = apiStatusLabel(result);
  }
  const cls = tone === "warn" || tone === "danger" ? `error-state ${tone}` : "error-state";
  return `<div class="${cls}" role="status">${escape(prefix ? `${prefix}: ${message}` : message)}</div>`;
}

// metricCard — the prominent KPI tile used by overview, observability,
// and any future dashboard.
export function metricCard({ eyebrow, icon, value, sub, tone = "neutral" } = {}) {
  const iconHtml = icon
    ? `<span class="grid h-8 w-8 place-items-center rounded-lg bg-paper"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-${escape(icon)}"/></svg></span>`
    : "";
  return `<article class="panel p-5">
    <div class="flex items-start justify-between"><span class="eyebrow">${escape(eyebrow || "")}</span>${iconHtml}</div>
    <div class="mt-6 flex items-end justify-between gap-3">
      <span class="metric-value">${escape(value)}</span>
      ${sub ? `<span class="status-pill ${toneClass(tone)}"><span class="status-dot"></span>${escape(sub)}</span>` : ""}
    </div>
  </article>`;
}

// metricGrid — wrap metricCard output in the responsive grid used
// everywhere KPI tiles appear.
export function metricGrid(cards) {
  if (!cards.length) return "";
  return `<div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">${cards.join("")}</div>`;
}

// chipGroup — filter / segment control. Used by audit, capabilities,
// operations, and any view that offers in-place filtering.
export function chipGroup(items, { activeKey, onClickDataAttr = "data-chip", size = "md" } = {}) {
  const sizeClass = size === "sm" ? "text-[.6rem]" : "text-[.66rem]";
  return `<div class="chip-group" role="tablist">${items.map((it) => {
    const on = it.key === activeKey;
    const label = it.label || it.key;
    return `<button type="button" class="chip ${on ? "active" : ""} ${sizeClass}" ${onClickDataAttr}="${escape(it.key)}" role="tab" aria-selected="${on}">${escape(label)}</button>`;
  }).join("")}</div>`;
}

// inlineBanner — non-modal alert rendered inside the main area, e.g.
// "This capability is read-only because it was archived."
export function inlineBanner({ tone = "info", message, action = "" } = {}) {
  if (!message) return "";
  const cls = ["inline-banner"];
  if (tone !== "neutral") cls.push(tone);
  return `<div class="${cls.join(" ")}" role="status">
    <svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-${tone === "danger" ? "warning" : tone === "warn" ? "warning" : "bell"}"/></svg>
    <span class="flex-1">${escape(message)}</span>
    ${action}
  </div>`;
}

// dataTable — wraps an array of rows in the shared <table class="data-table">
// wrapper.  The caller provides the column descriptors and the row
// renderer.  Empty states are rendered in place when rows is empty.
export function dataTable({ columns, rows, emptyMessage = "No entries.", emptyIcon = "icon-grid", rowAttrs, headClass = "" }) {
  const head = `<thead><tr class="${headClass}">${columns.map((c) => {
    const align = c.align === "right" ? "text-right" : c.align === "center" ? "text-center" : "text-left";
    return `<th class="${align}">${escape(c.label)}</th>`;
  }).join("")}</tr></thead>`;
  if (!rows.length) {
    return emptyState(emptyMessage, { icon: emptyIcon });
  }
  const body = `<tbody>${rows.map((row, idx) => {
    const attrs = rowAttrs ? rowAttrs(row, idx) : "";
    return `<tr ${attrs}>${columns.map((c) => {
      const align = c.align === "right" ? "text-right" : c.align === "center" ? "text-center" : "text-left";
      const content = c.render ? c.render(row) : (row[c.key] ?? "");
      return `<td class="${align}">${content}</td>`;
    }).join("")}</tr>`;
  }).join("")}</tbody>`;
  return `<div class="overflow-x-auto"><table class="data-table">${head}${body}</table></div>`;
}

// skeletonStack — typed placeholders for the brief window between
// "rendering" and "data resolved".  Most views use it during the
// initial paint so the layout doesn't jump.
export function skeletonStack({ lines = 3, width = "100%" } = {}) {
  return `<div class="panel p-5">
    <div class="skeleton h-3 w-24"></div>
    ${Array.from({ length: lines }).map(() => `<div class="skeleton mt-4 h-4" style="width:${escape(width)}"></div>`).join("")}
  </div>`;
}

// skeletonCard — the "loading card" used in KPI grids.
export function skeletonCard() {
  return `<div class="skeleton-card">
    <div class="skeleton h-3 w-20"></div>
    <div class="skeleton mt-4 h-10 w-32"></div>
    <div class="skeleton mt-4 h-3 w-24"></div>
  </div>`;
}

// badge — like statusPill but used for non-status labels (e.g. "v3",
// environment).  Renders as a neutral pill by default.
export function badge(text, { tone = "neutral" } = {}) {
  return statusPill(text, tone);
}

// confirm — render a standardized confirm body inside a modal. The
// caller provides the question and the consequence copy.
export function confirmBlock({ title, message, confirmLabel = "Confirm", confirmTone = "primary" } = {}) {
  return `<div class="modal-body">
    ${title ? `<h3 class="modal-title">${escape(title)}</h3>` : ""}
    ${message ? `<p class="modal-subtitle">${escape(message)}</p>` : ""}
  </div>
  <div class="modal-footer">
    <button type="button" class="quiet-button" data-close-modal>Cancel</button>
    <button type="button" class="${confirmTone === "danger" ? "danger-button" : "primary-button"}" data-confirm>${escape(confirmLabel)}</button>
  </div>`;
}

// progressBar — used for reputation gauges, eval progress, and any
// other 0..1 visual.
export function progressBar(value, { tone = "info", label = "" } = {}) {
  const pct = Math.max(0, Math.min(1, value)) * 100;
  const toneFill = { good: "#789c35", warn: "#d6a52a", danger: "#cc5a3f", info: "#6878ff" }[tone] || "#6878ff";
  return `<div>
    ${label ? `<div class="flex items-center justify-between"><span class="text-[.7rem] font-semibold">${escape(label)}</span><span class="mono text-[.66rem] text-muted">${pct.toFixed(0)}%</span></div>` : ""}
    <div class="mt-1 h-2 overflow-hidden rounded-full bg-paper"><div class="h-full rounded-full" style="width:${pct.toFixed(1)}%; background:${toneFill}"></div></div>
  </div>`;
}

export default {
  statusPill,
  pageHeader,
  panel,
  emptyState,
  errorState,
  metricCard,
  metricGrid,
  chipGroup,
  inlineBanner,
  dataTable,
  skeletonStack,
  skeletonCard,
  badge,
  confirmBlock,
  progressBar,
  TONES,
};
