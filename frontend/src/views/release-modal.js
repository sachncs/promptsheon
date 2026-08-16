// Release review modal. Opened from the releases table or any
// "Pending releases" card. Surfaces the manifest, approval tally,
// and the approve / reject / rollback / try-it controls.

import { escape, formatRelative, apiStatusLabel } from "../utils.js";
import * as api from "../api.js";
import { openModal, closeModal } from "../dialog.js";
import { statusPill, inlineBanner } from "../ui.js";
import { toast } from "../toast.js";

const STATUS_TONES = { active: "good", approved: "good", pending: "warn", superseded: "neutral", rolled_back: "danger" };
function statusTone(s) { return STATUS_TONES[s] || "neutral"; }

function statTile(label, value) {
  return `<div class="rounded-lg bg-paper p-3"><span class="eyebrow">${escape(label)}</span><span class="mono mt-2 block text-[.8rem] font-bold">${escape(value)}</span></div>`;
}

function renderManifest(manifest) {
  if (!manifest || !Object.keys(manifest).length) {
    return `<span class="text-muted">(empty manifest)</span>`;
  }
  return Object.entries(manifest).map(([key, value]) => {
    const text = typeof value === "string" ? value : JSON.stringify(value);
    return `<div class="flex items-baseline gap-2"><span class="text-muted shrink-0">${escape(key)}:</span> <span class="break-all">${escape(text)}</span></div>`;
  }).join("");
}

function renderApprovalSummary(approval) {
  if (!approval) {
    return `<p class="mt-2 text-[.68rem] text-muted">Approval tally unavailable — quorum check will fail until the API responds.</p>`;
  }
  const votes = approval.votes || [];
  const updated = approval.updated_at;
  return `<p class="mt-2 text-[.68rem] text-muted">${votes.length} vote${votes.length === 1 ? "" : "s"} cast${updated ? ` · updated ${escape(formatRelative(updated))}` : ""}.</p>`;
}

function renderVoteRow(vote) {
  const tone = vote.decision === "approve" ? "good" : vote.decision === "reject" ? "danger" : "neutral";
  const initial = (vote.identity || "?")[0].toUpperCase();
  return `<div class="flex items-center gap-3">
    <span class="grid h-8 w-8 place-items-center rounded-lg bg-paper text-[.62rem] font-bold">${escape(initial)}</span>
    <span class="flex-1 text-[.72rem]"><span class="font-bold">${escape(vote.identity)}</span><span class="block text-[.63rem] text-muted">${escape(vote.reason || "no reason")} · ${escape(formatRelative(vote.timestamp))}</span></span>
    ${statusPill(vote.decision, tone)}
  </div>`;
}

function renderVoteRows(approval) {
  if (!approval || !Array.isArray(approval.votes) || approval.votes.length === 0) {
    return `<p class="text-[.68rem] text-muted">No votes yet.</p>`;
  }
  return approval.votes.map(renderVoteRow).join("");
}

function modalSkeleton(releaseId) {
  return `<div>
    <span class="skeleton block h-3 w-40"></span>
    <span class="skeleton mt-4 block h-6 w-72"></span>
    <span class="skeleton mt-4 block h-4 w-56"></span>
    <span class="skeleton mt-4 block h-24 w-full"></span>
    <span class="skeleton mt-4 block h-12 w-full"></span>
  </div>`;
}

function releaseBodyHtml(release, capability, approval) {
  const r = release || {};
  const name = capability?.name || r.capability_id || "Unknown capability";
  const canVote = r.status === "pending";
  const canRollback = r.status === "active" || r.status === "superseded";
  const canInvoke = r.status === "active";
  return `<div>
    <p class="text-[.7rem] text-muted">v${escape(r.capability_version || "?")} · <span class="mono">${escape(r.id || "")}</span> · ${statusPill(r.status || "?", statusTone(r.status))}</p>
    ${renderApprovalSummary(approval)}
    <div class="mt-5 space-y-5">
      <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
        ${statTile("Version", r.capability_version ?? "—")}
        ${statTile("Environment", r.environment ?? "—")}
        ${statTile("Created", formatRelative(r.created_at))}
        ${statTile("Status", r.status ?? "—")}
      </div>
      <div>
        <div class="eyebrow">Manifest fingerprints</div>
        <div class="mt-2 max-h-32 overflow-auto rounded-lg bg-paper p-3 text-[.65rem] mono text-[#50535a]">${renderManifest(r.manifest)}</div>
      </div>
      <div>
        <div class="eyebrow">Approval activity</div>
        <div class="mt-2 space-y-2">
          ${renderVoteRows(approval)}
        </div>
      </div>
      <form id="vote-form" class="space-y-3" data-release-id="${escape(r.id || "")}">
        <label class="field-label" for="vote-note">Decision note</label>
        <textarea id="vote-note" class="field min-h-20 resize-y" placeholder="Add context for the audit trail"></textarea>
        <p id="vote-error" class="hidden"></p>
        <div id="invoke-result" class="hidden"></div>
        <div class="flex flex-col-reverse gap-2 pt-2 sm:flex-row sm:justify-end">
          ${canRollback ? `<button type="button" id="vote-rollback" data-rollback class="quiet-button">Rollback</button>` : ""}
          ${canInvoke ? `<button type="button" data-invoke class="quiet-button">Try it</button>` : ""}
          <button type="button" class="quiet-button" data-close-modal>Close</button>
          ${canVote ? `
            <button type="submit" name="decision" value="reject" class="quiet-button">Reject</button>
            <button type="submit" name="decision" value="approve" class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-check"/></svg>Record approval</button>
          ` : ""}
        </div>
      </form>
    </div>
  </div>`;
}

export async function openReleaseModal(releaseId) {
  const modal = openModal({
    title: `Release review`,
    subtitle: `Loading ${releaseId || ""}…`,
    body: modalSkeleton(releaseId),
    size: "wide",
  });
  const [release, approval] = await Promise.all([
    api.getRelease(releaseId),
    api.getReleaseApproval(releaseId),
  ]);
  const releaseData = release.ok ? release.data : null;
  let capabilityData = null;
  if (releaseData?.capability_id) {
    const c = await api.getCapability(releaseData.capability_id);
    if (c.ok) capabilityData = c.data;
  }
  if (!release.ok) {
    modal.body.innerHTML = `<p class="text-[.78rem] text-muted">${escape(apiStatusLabel(release))}</p>`;
    return;
  }
  const name = capabilityData?.name || releaseData?.capability_id || "Release";
  // Update subtitle + title now that data is loaded.
  const subtitle = modal.root.querySelector(".modal-subtitle");
  const title = modal.root.querySelector(".modal-title");
  if (subtitle) subtitle.textContent = `Release review · ${releaseData?.environment || "?"}`;
  if (title) title.textContent = name;

  modal.body.innerHTML = releaseBodyHtml(releaseData, capabilityData, approval.ok ? approval.data : null);
  attach(modal, releaseData);
}

function attach(modal, release) {
  const root = modal.root;
  const form = root.querySelector("#vote-form");
  form?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const submitter = event.submitter;
    const decision = submitter?.value || "approve";
    const reason = (root.querySelector("#vote-note")?.value || "").trim();
    const error = root.querySelector("#vote-error");
    const buttons = form.querySelectorAll('button[type="submit"], [data-rollback]');
    buttons.forEach((b) => (b.disabled = true));
    const response = await api.voteRelease(release.id, decision, reason);
    if (!response.ok) {
      buttons.forEach((b) => (b.disabled = false));
      if (error) {
        error.innerHTML = inlineBanner({ tone: "danger", message: apiStatusLabel(response) });
        error.classList.remove("hidden");
      }
      return;
    }
    if (decision === "approve") {
      await api.activateRelease(release.id);
    }
    modal.close();
    toast.success(decision === "approve" ? "Release approved" : "Release rejected", release.id);
    setTimeout(() => window.location.reload(), 250);
  });

  const rollback = root.querySelector("[data-rollback]");
  rollback?.addEventListener("click", async () => {
    const error = root.querySelector("#vote-error");
    rollback.disabled = true;
    const response = await api.rollbackRelease(release.id);
    rollback.disabled = false;
    if (!response.ok) {
      if (error) {
        error.innerHTML = inlineBanner({ tone: "danger", message: apiStatusLabel(response) });
        error.classList.remove("hidden");
      }
      return;
    }
    modal.close();
    toast.success("Release rolled back", release.id);
    setTimeout(() => window.location.reload(), 250);
  });

  const invoke = root.querySelector("[data-invoke]");
  invoke?.addEventListener("click", async () => {
    const error = root.querySelector("#vote-error");
    const note = root.querySelector("#vote-note");
    const result = root.querySelector("#invoke-result");
    if (error) { error.classList.add("hidden"); error.innerHTML = ""; }
    if (result) { result.classList.add("hidden"); result.innerHTML = ""; }
    invoke.disabled = true;
    const inputs = (() => {
      const raw = (note?.value || "").trim();
      if (!raw) return { q: "hello" };
      try { return JSON.parse(raw); } catch { return { q: raw }; }
    })();
    const response = await api.invokeRelease(release.id, inputs);
    invoke.disabled = false;
    if (result) {
      result.classList.remove("hidden");
      if (!response.ok) {
        result.innerHTML = inlineBanner({ tone: "danger", message: `Invoke failed: ${apiStatusLabel(response)}` });
      } else {
        result.innerHTML = inlineBanner({ tone: "good", message: `Invoked (${response.data?.status || "ok"}) · ${escape(JSON.stringify(response.data?.outputs || response.data || {}).slice(0, 200))}` });
      }
    }
  });
}
