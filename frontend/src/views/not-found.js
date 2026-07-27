import { escape } from "../utils.js";

export function renderNotFound(route) {
  return `<section class="panel p-8 text-center">
    <p class="text-[.78rem] font-bold">Route not found</p>
    <p class="mt-2 text-[.7rem] text-muted">Unknown route <span class="mono">${escape(route.path)}</span>.</p>
    <a href="#/" class="primary-button mt-4">Back to overview</a>
  </section>`;
}
