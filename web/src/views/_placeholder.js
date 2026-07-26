import { escape } from "../utils.js";

export function renderCapabilitiesListPlaceholder(route) {
  return `<section class="panel p-6">
    <p class="text-[.78rem] font-bold">Capabilities</p>
    <p class="mt-2 text-[.7rem] text-muted">Stub — landing in later commit.</p>
  </section>`;
}

export function renderCapabilityDetailPlaceholder(route) {
  return `<section class="panel p-6">
    <p class="text-[.78rem] font-bold">Capability detail</p>
    <p class="mt-2 text-[.7rem] text-muted">Stub — landing in later commit (id ${escape(route.params.id || "—")}).</p>
  </section>`;
}

export function renderReleasesPlaceholder(route) {
  return `<section class="panel p-6">
    <p class="text-[.78rem] font-bold">Releases</p>
    <p class="mt-2 text-[.7rem] text-muted">Stub.</p>
  </section>`;
}

export function renderAuditPlaceholder(route) {
  return `<section class="panel p-6">
    <p class="text-[.78rem] font-bold">Audit trail</p>
    <p class="mt-2 text-[.7rem] text-muted">Stub.</p>
  </section>`;
}

export function renderObservabilityPlaceholder(route) {
  return `<section class="panel p-6">
    <p class="text-[.78rem] font-bold">Observability</p>
    <p class="mt-2 text-[.7rem] text-muted">Stub.</p>
  </section>`;
}

export function renderGuardrailsPlaceholder(route) {
  return `<section class="panel p-6">
    <p class="text-[.78rem] font-bold">Guardrails</p>
    <p class="mt-2 text-[.7rem] text-muted">Stub.</p>
  </section>`;
}

export function renderEvaluationsPlaceholder(route) {
  return `<section class="panel p-6">
    <p class="text-[.78rem] font-bold">Evaluations</p>
    <p class="mt-2 text-[.7rem] text-muted">Stub.</p>
  </section>`;
}

export function renderLogsPlaceholder(route) {
  return `<section class="panel p-6">
    <p class="text-[.78rem] font-bold">Live logs</p>
    <p class="mt-2 text-[.7rem] text-muted">Stub.</p>
  </section>`;
}

export function renderOperationsPlaceholder(route) {
  return `<section class="panel p-6">
    <p class="text-[.78rem] font-bold">Operations</p>
    <p class="mt-2 text-[.7rem] text-muted">Stub (tab: ${escape(route.params.tab || "—")}).</p>
  </section>`;
}
