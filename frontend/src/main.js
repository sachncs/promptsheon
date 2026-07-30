import "./styles.css";
import * as api from "./api.js";
import { loadSettings, watchSettings } from "./settings.js";
import { initRouter, go, currentRoute } from "./router.js";
import { renderAppShell, renderInitialState } from "./shell.js";
import { renderView } from "./views/index.js";
import { ensureOwnerIndex } from "./state/owners.js";

window.promptsheon = { api, go };

const qs = (selector, scope = document) => scope.querySelector(selector);

function bootstrap() {
  const root = qs("#app");
  root.innerHTML = renderAppShell();
  initRouter({
    onChange: () => {
      const route = currentRoute();
      window.document.querySelectorAll("[data-nav]").forEach((el) => {
        el.classList.toggle("active", el.getAttribute("href") === `#${route.path}`);
      });
      const subnav = window.document.querySelectorAll("[data-subnav]");
      const operationsMatch = ["operations", "operations/alerts", "operations/webhooks", "operations/vault", "operations/providers", "operations/users", "operations/reasoning"];
      subnav.forEach((el) => {
        const match = el.getAttribute("href").slice(2);
        const isOps = operationsMatch.includes(route.path);
        el.classList.toggle("active", match === route.path || (isOps && match === "operations"));
      });
      renderView(route);
    }
  });
  renderInitialState();
  // Owners depend on the API key; if no key is set yet, skip the fetch
  // and let watchSettings retry once the user pastes a key.
  if (loadSettings().apiKey) ensureOwnerIndex();
  watchSettings(() => {
    renderView(currentRoute());
    if (loadSettings().apiKey) ensureOwnerIndex();
  });
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", bootstrap);
} else {
  bootstrap();
}
