// src/main.js — dashboard bootstrap.
//
// Mounts the shell, wires the router, kicks off the owner index,
// and observes connection settings so the dashboard re-renders when a
// user pastes an API key.

import "./styles.css";
import * as api from "./api.js";
import { loadSettings, watchSettings } from "./settings.js";
import { initRouter, go, currentRoute } from "./router.js";
import { mountShell } from "./shell.js";
import { renderView } from "./views/index.js";
import { ensureOwnerIndex } from "./state/owners.js";

// Expose a tiny helper namespace so embedded docs / smoke tests can
// poke the app from devtools: `promptsheon.app.go("/audit")`.
window.promptsheon = { api, go, app: { go } };

function bootstrap() {
  initRouter({
    onChange: () => {
      const route = currentRoute();
      window.document.querySelectorAll("[data-nav]").forEach((el) => {
        el.classList.toggle("active", el.getAttribute("href") === `#${route.path}`);
      });
      renderView(route);
    },
  });

  mountShell();

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
