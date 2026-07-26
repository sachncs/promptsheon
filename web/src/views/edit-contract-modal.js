import { escape, apiStatusLabel } from "../utils.js";
import * as api from "../api.js";

export async function openEditContractModal(root, capability) {
  if (!root) return;
  const contractRes = await api.getCapabilityContract(capability.id);
  const initial = contractRes.ok ? contractRes.data || { slo_target: {}, blast_radius: "low", auto_promotable: false } : { slo_target: {}, blast_radius: "low", auto_promotable: false };
  const slo = initial.slo_target || {};
  const render = ({ error = null, saved = null } = {}) => `<div class="modal-backdrop" role="presentation">
    <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="edit-contract-title">
      <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
        <div><div class="eyebrow">Contract / Edit</div><h2 id="edit-contract-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">${escape(capability.name)} contract</h2><p class="mt-1 text-[.7rem] text-muted">Defines the SLO, blast radius, and schema required to auto-promote releases.</p></div>
        <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
      </div>
      <form id="edit-contract-form" class="space-y-4 px-5 py-5 sm:px-6">
        <div><label class="eyebrow mb-2 block" for="ec-rubric">Success rubric</label><textarea id="ec-rubric" name="success_rubric" class="field min-h-20 resize-y" data-autofocus>${escape(initial.success_rubric || "")}</textarea></div>
        <div class="grid gap-3 sm:grid-cols-2">
          <div><label class="eyebrow mb-2 block" for="ec-blast">Blast radius</label><select id="ec-blast" name="blast_radius" class="field"><option value="low" ${initial.blast_radius === "low" ? "selected" : ""}>low</option><option value="medium" ${initial.blast_radius === "medium" ? "selected" : ""}>medium</option><option value="high" ${initial.blast_radius === "high" ? "selected" : ""}>high</option></select></div>
          <div><label class="eyebrow mb-2 block" for="ec-auto">Auto-promotable</label><select id="ec-auto" name="auto_promotable" class="field"><option value="false" ${!initial.auto_promotable ? "selected" : ""}>false</option><option value="true" ${initial.auto_promotable ? "selected" : ""}>true</option></select></div>
          <div><label class="eyebrow mb-2 block" for="ec-p95">Max p95 latency (ms)</label><input id="ec-p95" name="max_p95_latency_ms" class="field mono" type="number" min="0" value="${escape(slo.max_p95_latency_ms ?? "")}" /></div>
          <div><label class="eyebrow mb-2 block" for="ec-success">Min success rate (0-1)</label><input id="ec-success" name="min_success_rate" class="field mono" type="number" step="0.01" min="0" max="1" value="${escape(slo.min_success_rate ?? "")}" /></div>
          <div><label class="eyebrow mb-2 block" for="ec-hallu">Max hallucination rate (0-1)</label><input id="ec-hallu" name="max_hallucination_rate" class="field mono" type="number" step="0.01" min="0" max="1" value="${escape(slo.max_hallucination_rate ?? "")}" /></div>
        </div>
        <details class="rounded-lg border border-line p-3"><summary class="cursor-pointer text-[.66rem] font-bold">Input schema (JSON)</summary><textarea id="ec-input" name="input_schema" class="field mono mt-2 min-h-24 resize-y" placeholder='{"type":"object"}'>${escape(formatJson(initial.input_schema))}</textarea></details>
        <details class="rounded-lg border border-line p-3"><summary class="cursor-pointer text-[.66rem] font-bold">Output schema (JSON)</summary><textarea id="ec-output" name="output_schema" class="field mono mt-2 min-h-24 resize-y" placeholder='{"type":"object"}'>${escape(formatJson(initial.output_schema))}</textarea></details>
        ${error ? `<p class="rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800">${escape(error)}</p>` : ""}
        ${saved ? `<p class="rounded-lg bg-lime/15 px-3 py-2 text-[.68rem] text-[#52632d]">Contract saved.</p>` : ""}
        <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
          <button type="button" class="quiet-button" data-close-modal>Cancel</button>
          <button type="submit" class="primary-button">Save contract</button>
        </div>
      </form>
    </section>
  </div>`;
  root.innerHTML = render();
  attach(root, render, capability);
}

function formatJson(value) {
  if (!value) return "";
  try { return JSON.stringify(value, null, 2); }
  catch { return ""; }
}

function attach(root, render, capability) {
  root.querySelector("[data-close-modal]")?.addEventListener("click", () => root.replaceChildren());
  root.querySelector("#edit-contract-form")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const data = new FormData(event.target);
    const payload = {
      success_rubric: (data.get("success_rubric") || "").toString().trim() || undefined,
      blast_radius: (data.get("blast_radius") || "low").toString(),
      auto_promotable: data.get("auto_promotable") === "true",
      slo_target: {
        max_p95_latency_ms: data.get("max_p95_latency_ms") ? Number(data.get("max_p95_latency_ms")) : undefined,
        min_success_rate: data.get("min_success_rate") ? Number(data.get("min_success_rate")) : undefined,
        max_hallucination_rate: data.get("max_hallucination_rate") ? Number(data.get("max_hallucination_rate")) : undefined
      }
    };
    const inputSchema = parseSchema(data.get("input_schema"));
    const outputSchema = parseSchema(data.get("output_schema"));
    if (inputSchema) payload.input_schema = inputSchema;
    if (outputSchema) payload.output_schema = outputSchema;
    const result = await api.updateCapabilityContract(capability.id, payload);
    if (!result.ok) {
      root.innerHTML = render({ error: apiStatusLabel(result) });
      attach(root, render, capability);
      return;
    }
    window.location.reload();
  });
}

function parseSchema(raw) {
  if (!raw) return null;
  try { return JSON.parse(raw); }
  catch { return null; }
}
