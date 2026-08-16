// src/dialog.js — uniform modal/dialog system.
//
// Goals:
//   - One openModal() / closeModal() surface so every view follows
//     the same lifecycle.
//   - Focus trap + restore so screen-reader and keyboard users get a
//     consistent experience (open: trap focus inside; close: return
//     focus to the trigger).
//   - ESC + backdrop click close with a single global listener.
//   - Accessible: aria-modal, aria-labelledby, role="dialog", scroll
//     lock while open.
//
// API:
//   openModal({ title, subtitle, body, footer, size, onClose, dismissible })
//     returns { close, root, body }
//   closeModal(root) — closes a specific modal; safe to call multiple
//     times.

import { escape } from "./utils.js";

const LISTENERS = new WeakMap();
let stack = []; // newest first

function ensureRoot() {
  let root = document.getElementById("modal-root");
  if (!root) {
    root = document.createElement("div");
    root.id = "modal-root";
    document.body.appendChild(root);
  }
  return root;
}

function focusableElements(container) {
  return Array.from(
    container.querySelectorAll(
      'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]):not([type="hidden"]), select:not([disabled]), [tabindex]:not([tabindex="-1"])',
    ),
  ).filter((el) => el.offsetWidth > 0 || el.offsetHeight > 0 || el === document.activeElement);
}

function trapFocus(root, event) {
  if (event.key !== "Tab") return;
  const focusables = focusableElements(root);
  if (focusables.length === 0) {
    event.preventDefault();
    return;
  }
  const first = focusables[0];
  const last = focusables[focusables.length - 1];
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault();
    last.focus();
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault();
    first.focus();
  }
}

function lockScroll() {
  if (stack.length === 0) {
    document.documentElement.style.overflow = "hidden";
    document.body.style.overflow = "hidden";
  }
}

function unlockScroll() {
  if (stack.length === 0) {
    document.documentElement.style.overflow = "";
    document.body.style.overflow = "";
  }
}

function attach(root, opts) {
  const dismissible = opts.dismissible !== false;
  const previouslyFocused = document.activeElement;

  function onKeydown(event) {
    if (event.key === "Escape" && dismissible) {
      event.preventDefault();
      closeModal(root);
      return;
    }
    trapFocus(root, event);
  }

  function onBackdrop(event) {
    if (!dismissible) return;
    if (event.target === root || event.target.classList.contains("modal-backdrop")) {
      closeModal(root);
    }
  }

  function onCloseClick(event) {
    const closer = event.target.closest("[data-close-modal]");
    if (closer) {
      event.preventDefault();
      closeModal(root);
    }
  }

  root.addEventListener("keydown", onKeydown);
  root.addEventListener("mousedown", onBackdrop);
  root.addEventListener("click", onCloseClick);
  LISTENERS.set(root, { onKeydown, onBackdrop, onCloseClick });

  lockScroll();
  stack.unshift({ root, previouslyFocused, onClose: opts.onClose });

  // Focus the first focusable element (or the modal itself if none).
  const first = focusableElements(root)[0];
  if (first) {
    setTimeout(() => first.focus(), 0);
  } else {
    root.tabIndex = -1;
    root.focus();
  }

  return previouslyFocused;
}

function detach(root) {
  const meta = LISTENERS.get(root);
  if (!meta) return;
  root.removeEventListener("keydown", meta.onKeydown);
  root.removeEventListener("mousedown", meta.onBackdrop);
  root.removeEventListener("click", meta.onCloseClick);
  LISTENERS.delete(root);
}

export function openModal(opts = {}) {
  const root = ensureRoot();
  const {
    title,
    subtitle,
    body = "",
    footer = "",
    size, // "narrow" | "wide" | undefined
    onClose,
    dismissible = true,
    ariaLabel,
  } = opts;

  const sizeClass = size === "wide" ? "wide" : size === "narrow" ? "narrow" : "";
  const labelAttr = ariaLabel ? `aria-label="${escape(ariaLabel)}"` : title ? `aria-labelledby="modal-title"` : "";

  root.innerHTML = `
    <div class="modal-backdrop" role="presentation">
      <div class="modal-card ${sizeClass}" role="dialog" aria-modal="true" ${labelAttr} tabindex="-1">
        ${title || opts.dismissible !== false
          ? `<div class="modal-header">
               <div class="min-w-0">
                 ${title ? `<h2 id="modal-title" class="modal-title">${escape(title)}</h2>` : ""}
                 ${subtitle ? `<p class="modal-subtitle">${escape(subtitle)}</p>` : ""}
               </div>
               ${opts.dismissible !== false
                 ? `<button type="button" class="icon-button" aria-label="Close" data-close-modal><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>`
                 : ""}
             </div>`
          : ""}
        <div class="modal-body">${body}</div>
        ${footer ? `<div class="modal-footer">${footer}</div>` : ""}
      </div>
    </div>
  `;

  attach(root, { onClose, dismissible });

  return {
    root,
    card: root.querySelector(".modal-card"),
    body: root.querySelector(".modal-body"),
    close: () => closeModal(root),
  };
}

export function closeModal(root) {
  const modalRoot = root && root.classList ? root : document.getElementById("modal-root");
  if (!modalRoot) return;

  const entry = stack.find((s) => s.root === modalRoot);
  detach(modalRoot);
  modalRoot.innerHTML = "";
  stack = stack.filter((s) => s.root !== modalRoot);
  unlockScroll();

  if (entry && entry.previouslyFocused && typeof entry.previouslyFocused.focus === "function") {
    try {
      entry.previouslyFocused.focus();
    } catch (_) {
      /* the trigger may have been removed from the DOM; ignore. */
    }
  }

  if (entry && typeof entry.onClose === "function") {
    try { entry.onClose(); } catch (_) { /* caller errors don't block close */ }
  }
}

export function closeAllModals() {
  while (stack.length) closeModal(stack[0].root);
}

// confirmModal — uniform confirmation dialog. Returns a Promise<boolean>
// so callers can await the user's choice without a custom handler.
export function confirmModal({ title, message, confirmLabel = "Confirm", cancelLabel = "Cancel", confirmTone = "primary", dismissible = true } = {}) {
  return new Promise((resolve) => {
    const root = ensureRoot();
    const buttons = `
      <button type="button" class="quiet-button" data-confirm-cancel>${escape(cancelLabel)}</button>
      <button type="button" class="${confirmTone === "danger" ? "danger-button" : "primary-button"}" data-confirm-ok>${escape(confirmLabel)}</button>
    `;
    openModal({
      title,
      subtitle: message,
      size: "narrow",
      footer: buttons,
      dismissible,
      onClose: () => resolve(false),
    });
    const ok = root.querySelector("[data-confirm-ok]");
    const cancel = root.querySelector("[data-confirm-cancel]");
    ok.addEventListener("click", () => {
      resolve(true);
      closeModal(root);
    });
    cancel.addEventListener("click", () => {
      resolve(false);
      closeModal(root);
    });
  });
}

export default { openModal, closeModal, closeAllModals, confirmModal };
