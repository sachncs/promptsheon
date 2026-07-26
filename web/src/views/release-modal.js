import { escape, formatRelative, apiStatusLabel } from "../utils.js";
import * as api from "../api.js";

function modalSkeleton(releaseId) {
  return `<div class="modal-backdrop" role="presentation">
    <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="release-title">
      <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
        <div><div class="eyebrow">Release review</div><h2 id="release-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">Loading ${escape(releaseId || "")}…</h2></div>
        <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
      </div>
      <div class="px-6 py-6 space-y-3">
        <span class="skeleton block h-4 w-40"></span>
        <span class="skeleton block h-4 w-72"></span>
        <span class="skeleton block h-4 w-56"></span>
        <span class="skeleton block h-24 w-full"></span>
      </div>
    </section>
  </div>`;
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

function renderApproval(approval) {
  if (!approval) {
    return `<p class="mt-2 text-[.68rem] text-muted">Approval tally unavailable — quorum check will fail until the API responds.</p>`;
  }
  const votes = approval.votes || [];
  const updated = approval.updated_at;
  return `<p class="mt-2 text-[.68rem] text-muted">${votes.length} vote${votes.length === 1 ? "" : "s"} cast${updated ? ` · updated ${escape(formatRelative(updated))}` : ""}.</p>`;
}

function releaseModalHtml(release, capability, approval) {
  const r = release || {};
  const name = capability?.name || r.capability_id || "Unknown capability";
  const tone = ({ active: "good", approved: "good", pending: "warn", superseded: "neutral", rolled_back: "danger" })[r.status] || "neutral";
  const canVote = r.status === "pending";
  const canRollback = r.status === "active" || r.status === "superseded";
  return `<div class="modal-backdrop" role="presentation">
    <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="release-title">
      <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
        <div>
          <div class="eyebrow">Release review · ${escape(r.environment || "?")}</div>
          <h2 id="release-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">${escape(name)}</h2>
          <p class="mt-1 text-[.7rem] text-muted">v${escape(r.capability_version || "?")} · <span class="mono">${escape(r.id || "")}</span> · ${statusPillHtml(r.status || "?", tone)}</p>
          ${renderApproval(approval)}
        </div>
        <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
      </div>
      <div class="space-y-5 px-5 py-5 sm:px-6">
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
            ${renderVoteRow("Required approver", "?", "Pending", "warn")}
          </div>
        </div>
        <form id="vote-form" class="space-y-3" data-release-id="${escape(r.id || "")}">
          <label class="eyebrow block" for="vote-note">Decision note</label>
          <textarea id="vote-note" class="field min-h-20 resize-y" placeholder="Add context for the audit trail"></textarea>
          <p id="vote-error" class="hidden rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800"></p>
          <div class="flex flex-col-reverse gap-2 pt-2 sm:flex-row sm:justify-end">
            ${canRollback ? `<button type="button" id="vote-rollback" data-rollback class="quiet-button">Rollback</button>` : ""}
            <button type="button" class="quiet-button" data-close-modal>Close</button>
            ${canVote ? `
              <button type="submit" name="decision" value="reject" class="quiet-button">Reject</button>
              <button type="submit" name="decision" value="approve" class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-check"/></svg>Record approval</button>
            ` : ""}
          </div>
        </form>
      </div>
    </section>
  </div>`;
}

function renderVoteRows(approval) {
  if (!approval || !Array.isArray(approval.votes) || approval.votes.length === 0) {
    return `<p class="text-[.68rem] text-muted">No votes yet.</p>`;
  }
  return approval.votes.map((vote) => {
    const tone = vote.decision === "approve" ? "good" : vote.decision === "reject" ? "danger" : "neutral";
    const initial = (vote.identity || "?")[0].toUpperCase();
    return `<div class="flex items-center gap-3">
      <span class="grid h-8 w-8 place-items-center rounded-lg bg-paper text-[.62rem] font-bold">${escape(initial)}</span>
      <span class="flex-1 text-[.72rem]"><span class="font-bold">${escape(vote.identity)}</span><span class="block text-[.63rem] text-muted">${escape(vote.reason || "no reason")} · ${escape(formatRelative(vote.timestamp))}</span></span>
      ${statusPillHtml(vote.decision, tone)}
    </div>`;
  }).join("");
}

function renderVoteRow(label, initial, status, tone) {
  return `<div class="flex items-center gap-3">
    <span class="grid h-8 w-8 place-items-center rounded-lg border border-dashed border-[#c9cac5] text-[.7rem] text-muted">${escape(initial)}</span>
    <span class="flex-1 text-[.72rem]"><span class="font-bold">${escape(label)}</span><span class="block text-[.63rem] text-muted">${escape(status)}</span></span>
    ${statusPillHtml(status, tone)}
  </div>`;
}

function statTile(label, value) {
  return `<div class="rounded-lg bg-paper p-3"><span class="eyebrow">${escape(label)}</span><span class="mono mt-2 block text-[.8rem] font-bold">${escape(value)}</span></div>`;
}

function statusPillHtml(label, tone) {
  return `<span class="status-pill ${tone} !px-2 !py-1"><span class="status-dot"></span>${escape(label)}</span>`;
}

async function open(root, releaseId) {
  root.innerHTML = modalSkeleton(releaseId);
  const [release, capability, approval] = await Promise.all([
    api.getRelease(releaseId),
    api.getCapability(/* placeholder */ null).catch(() => null),
    api.getReleaseApproval(releaseId)
  ]);
  const releaseData = release.ok ? release.data : null;
  let capabilityData = null;
  if (releaseData?.capability_id) {
    const c = await api.getCapability(releaseData.capability_id);
    if (c.ok) capabilityData = c.data;
  }
  if (!release.ok) {
    root.innerHTML = `<div class="modal-backdrop" role="presentation"><section class="modal-card" role="dialog" aria-modal="true"><div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6"><div><div class="eyebrow">Release</div><h2 class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">Cannot load release</h2></div><button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button></div><div class="px-6 py-6 text-[.78rem] text-muted">${escape(apiStatusLabel(release))}</div></section></div>`;
    return;
  }
  root.innerHTML = releaseModalHtml(releaseData, capabilityData, approval.ok ? approval.data : null);
  attach(root, releaseData);
}

function attach(root, release) {
  const form = root.querySelector("#vote-form");
  form?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const submitter = event.submitter;
    const decision = submitter?.value || "approve";
    const reason = (root.querySelector("#vote-note")?.value || "").trim();
    const error = root.querySelector("#vote-error");
    const buttons = form.querySelectorAll("button[type=submit], [data-rollback]");
    buttons.forEach((b) => (b.disabled = true));
    const response = await api.voteRelease(release.id, decision, reason);
    if (!response.ok) {
      buttons.forEach((b) => (b.disabled = false));
      if (error) {
        error.textContent = apiStatusLabel(response);
        error.classList.remove("hidden");
      }
      return;
    }
    if (decision === "approve") {
      await api.activateRelease(release.id);
    }
    closeAndReload(form, root);
  });

  const rollback = root.querySelector("[data-rollback]");
  rollback?.addEventListener("click", async () => {
    const error = root.querySelector("#vote-error");
    rollback.disabled = true;
    const response = await api.rollbackRelease(release.id);
    rollback.disabled = false;
    if (!response.ok) {
      if (error) {
        error.textContent = apiStatusLabel(response);
        error.classList.remove("hidden");
      }
      return;
    }
    closeAndReload(form, root);
  });

  const close = root.querySelector("[data-close-modal]");
  close?.addEventListener("click", () => root.replaceChildren());
}

function closeAndReload(form, root) {
  root.replaceChildren();
  window.location.reload();
}

export async function openReleaseModal(root, releaseId) {
  if (!root) return;
  await open(root, releaseId);
}
