// First-run setup modal. Triggered when the dashboard has no
// API key AND the daemon's /api/v1/setup endpoint is reachable.
// Walks the operator through minting the initial admin key via
// POST /api/v1/setup, then stores the returned key like any other.
import * as api from "../api.js";
import { saveSettings, loadSettings } from "../settings.js";
import { escape, apiStatusLabel } from "../utils.js";

export async function openFirstRunSetupModal(root, { onReady } = {}) {
  function render(opts = {}) {
    return `<div class="modal-backdrop" role="presentation">
      <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="firstrun-title">
        <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
          <div>
            <div class="eyebrow">First-run setup</div>
            <h2 id="firstrun-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">Create your admin key</h2>
            <p class="mt-1 text-[.7rem] text-muted">No API key is configured. Mint the initial admin key against the daemon's bootstrap endpoint.</p>
          </div>
          <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
        </div>
        <form id="firstrun-form" class="space-y-3 px-5 py-5 sm:px-6">
          ${opts.error ? `<p class="rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800">${escape(opts.error)}</p>` : ""}
          <div>
            <label class="eyebrow mb-2 block" for="firstrun-email">Email</label>
            <input id="firstrun-email" name="email" type="email" class="field" required autofocus placeholder="admin@example.com" />
          </div>
          <div>
            <label class="eyebrow mb-2 block" for="firstrun-name">Name</label>
            <input id="firstrun-name" name="name" class="field" required placeholder="Admin" />
          </div>
          <div>
            <label class="eyebrow mb-2 block" for="firstrun-token">Bootstrap token</label>
            <input id="firstrun-token" name="bootstrapToken" class="field mono" required placeholder="value of PROMPTSHEON_BOOTSTRAP_TOKEN" />
            <p class="mt-1 text-[.62rem] text-muted">Set on the daemon: <code>PROMPTSHEON_BOOTSTRAP_TOKEN=&lt;random&gt;</code>. Required to prevent anonymous first-caller-wins admin minting.</p>
          </div>
          <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
            <button type="button" class="quiet-button" data-close-modal>Cancel</button>
            <button type="submit" class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-spark"/></svg>Create admin key</button>
          </div>
        </form>
        <div id="firstrun-created" class="hidden px-5 pb-5 sm:px-6"></div>
      </section>
    </div>`;
  }

  function attach() {
    root.querySelector("[data-close-modal]")?.addEventListener("click", () => root.replaceChildren());
    root.querySelector("#firstrun-form")?.addEventListener("submit", async (event) => {
      event.preventDefault();
      const data = new FormData(event.target);
      const email = String(data.get("email") || "").trim();
      const name = String(data.get("name") || "").trim();
      const bootstrapToken = String(data.get("bootstrapToken") || "").trim();
      if (!email || !name || !bootstrapToken) {
        root.innerHTML = render({ error: "Email, name, and bootstrap token are all required." });
        attach();
        return;
      }
      const result = await api.setupBootstrap({ email, name, bootstrapToken });
      if (!result.ok) {
        root.innerHTML = render({ error: `Setup failed: ${escape(apiStatusLabel(result))}` });
        attach();
        return;
      }
      const key = result.data?.key;
      if (!key) {
        root.innerHTML = render({ error: "Setup succeeded but no key was returned." });
        attach();
        return;
      }
      saveSettings({ apiKey: key });
      const createdSlot = root.querySelector("#firstrun-created");
      createdSlot.innerHTML = `<div class="rounded-xl border border-line bg-amber-50 p-4">
        <p class="text-[.74rem] font-bold">Admin key created.</p>
        <p class="mt-1 text-[.66rem] text-amber-800">Saved to browser storage. You can now close this dialog.</p>
      </div>`;
      createdSlot.classList.remove("hidden");
      // Hide the form now that we have a key.
      root.querySelector("#firstrun-form")?.classList.add("hidden");
      root.querySelector("[data-close-modal]")?.classList.remove("hidden");
      if (typeof onReady === "function") onReady(key);
    });
  }

  root.innerHTML = render();
  attach();
}

// shouldOfferFirstRun returns true when the dashboard has no
// API key. Caller can use this to decide whether to surface the
// first-run prompt on load.
export function shouldOfferFirstRun() {
  const settings = loadSettings();
  return !settings.apiKey;
}