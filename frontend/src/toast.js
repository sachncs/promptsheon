// src/toast.js — transient bottom-right feedback for create/update/delete.
//
// Goals:
//   - Stack-independent: every call shows a separate toast.
//   - Auto-dismiss after a duration (default 4.5s); pauses on hover.
//   - Tone-driven: good | warn | danger | info | neutral.
//   - Accessible: role="status", aria-live="polite".

import { escape } from "./utils.js";

const ICONS = {
  good: "icon-check",
  warn: "icon-warning",
  danger: "icon-warning",
  info: "icon-bell",
  neutral: "icon-bell",
};

function ensureStack() {
  let stack = document.getElementById("toast-stack");
  if (!stack) {
    stack = document.createElement("div");
    stack.id = "toast-stack";
    stack.className = "toast-stack";
    stack.setAttribute("role", "region");
    stack.setAttribute("aria-label", "Notifications");
    document.body.appendChild(stack);
  }
  return stack;
}

export function showToast({ title, message = "", tone = "info", duration = 4500 } = {}) {
  if (!title && !message) return null;
  const stack = ensureStack();
  const node = document.createElement("div");
  const safeTone = ["good", "warn", "danger", "info", "neutral"].includes(tone) ? tone : "info";
  node.className = `toast ${safeTone}`;
  node.setAttribute("role", "status");
  node.innerHTML = `
    <span class="toast-icon"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-[1.8]"><use href="#${escape(ICONS[safeTone] || "icon-bell")}"/></svg></span>
    <div class="min-w-0 flex-1">
      ${title ? `<p class="toast-title">${escape(title)}</p>` : ""}
      ${message ? `<p class="toast-message">${escape(message)}</p>` : ""}
    </div>
    <button type="button" class="toast-close" aria-label="Dismiss"><svg class="h-3 w-3 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
  `;
  stack.appendChild(node);

  let dismissTimer = setTimeout(() => dismiss(node), duration);

  node.addEventListener("mouseenter", () => clearTimeout(dismissTimer));
  node.addEventListener("mouseleave", () => {
    dismissTimer = setTimeout(() => dismiss(node), Math.min(2000, duration));
  });

  node.querySelector(".toast-close").addEventListener("click", () => dismiss(node));

  return {
    close: () => dismiss(node),
    node,
  };
}

function dismiss(node) {
  if (!node || !node.parentNode) return;
  node.style.transition = "opacity 160ms ease, transform 160ms ease";
  node.style.opacity = "0";
  node.style.transform = "translateY(4px)";
  setTimeout(() => {
    if (node.parentNode) node.parentNode.removeChild(node);
  }, 180);
}

export function dismissAllToasts() {
  const stack = document.getElementById("toast-stack");
  if (!stack) return;
  Array.from(stack.children).forEach(dismiss);
}

export const toast = {
  success: (title, message, opts) => showToast({ tone: "good", title, message, ...opts }),
  warning: (title, message, opts) => showToast({ tone: "warn", title, message, ...opts }),
  danger:  (title, message, opts) => showToast({ tone: "danger", title, message, ...opts }),
  info:    (title, message, opts) => showToast({ tone: "info", title, message, ...opts }),
  show:    showToast,
};

export default toast;
