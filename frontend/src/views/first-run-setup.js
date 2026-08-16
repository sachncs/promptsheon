// First-run setup modal. Triggered when the dashboard has no
// API key AND the daemon's /api/v1/setup endpoint is reachable.
// Walks the operator through minting the initial admin key via
// POST /api/v1/setup, then stores the returned key like any other.

import * as api from "../api.js";
import { saveSettings, loadSettings } from "../settings.js";
import { inlineBanner } from "../ui.js";
import { openModal } from "../dialog.js";

function formHtml({ error = "" } = {}) {
  return `${error ? inlineBanner({ tone: "danger", message: error }) : ""}
    <form id="firstrun-form" class="space-y-3">
      <div>
        <label class="field-label" for="firstrun-email">Email</label>
        <input id="firstrun-email" name="email" type="email" class="field" required autofocus placeholder="admin@example.com" />
      </div>
      <div>
        <label class="field-label" for="firstrun-name">Name</label>
        <input id="firstrun-name" name="name" class="field" required placeholder="Admin" />
      </div>
      <div>
        <label class="field-label" for="firstrun-token">Bootstrap token</label>
        <input id="firstrun-token" name="bootstrapToken" class="field mono" required placeholder="value of PROMPTSHEON_BOOTSTRAP_TOKEN" />
        <p class="mt-1 text-[.62rem] text-muted">Set on the daemon: <code>PROMPTSHEON_BOOTSTRAP_TOKEN=&lt;random&gt;</code>. Required to prevent anonymous first-caller-wins admin minting.</p>
      </div>
    </form>`;
}

function footerHtml() {
  return `<button type="button" class="quiet-button" data-close-modal>Cancel</button>
    <button type="submit" form="firstrun-form" class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-spark"/></svg>Create admin key</button>`;
}

export async function openFirstRunSetupModal(root, { onReady } = {}) {
  const modal = openModal({
    title: "Create your admin key",
    subtitle: "No API key is configured. Mint the initial admin key against the daemon's bootstrap endpoint.",
    body: formHtml(),
    footer: footerHtml(),
    size: "narrow",
  });

  function attach() {
    modal.root.querySelector("#firstrun-form")?.addEventListener("submit", async (event) => {
      event.preventDefault();
      const data = new FormData(event.target);
      const email = String(data.get("email") || "").trim();
      const name = String(data.get("name") || "").trim();
      const bootstrapToken = String(data.get("bootstrapToken") || "").trim();
      if (!email || !name || !bootstrapToken) {
        modal.body.innerHTML = formHtml({ error: "Email, name, and bootstrap token are all required." });
        attach();
        return;
      }
      const result = await api.setupBootstrap({ email, name, bootstrapToken });
      if (!result.ok) {
        modal.body.innerHTML = formHtml({ error: `Setup failed: ${apiStatusLabel(result)}` });
        attach();
        return;
      }
      const key = result.data?.key;
      if (!key) {
        modal.body.innerHTML = formHtml({ error: "Setup succeeded but no key was returned." });
        attach();
        return;
      }
      saveSettings({ apiKey: key });
      modal.body.innerHTML = inlineBanner({ tone: "good", message: "Admin key created — saved to browser storage. You can now close this dialog." });
      modal.root.querySelector("#firstrun-form")?.classList.add("hidden");
      modal.root.querySelector(".modal-footer button[type=submit]")?.classList.add("hidden");
      if (typeof onReady === "function") onReady(key);
    });
  }
  attach();
}

// shouldOfferFirstRun returns true when the dashboard has no
// API key. Caller can use this to decide whether to surface the
// first-run prompt on load.
export function shouldOfferFirstRun() {
  const settings = loadSettings();
  return !settings.apiKey;
}
