# Phase 9 — Production hardening and throughput

## Objective

Prove that Promptsheon is reliable, secure, recoverable, and sufficiently
throughput-oriented for its intended deployment profile.

## Responsible

- SRE/platform: capacity, deployment, availability, recovery, observability.
- Security: threat model, penetration testing, dependency and secret controls.
- Backend/runtime: performance, database, queue, provider, and API hardening.
- Frontend: performance, accessibility, browser compatibility.
- Engineering lead: release readiness and risk acceptance.

## Implementation

- Define service-level objectives and error budgets.
- Add production configuration validation and fail-fast startup checks.
- Enforce authentication, authorization, organization isolation, rate limits,
  payload limits, and secret handling.
- Add database backup, restore, migration, and integrity verification.
- Add CAS verification and repair procedures.
- Add graceful shutdown, worker draining, and deployment readiness gates.
- Add container scanning, SBOM generation, dependency audits, and secret scans.
- Add deployment rollback and database migration rollback strategy.
- Add capacity controls for API, queue, workers, SQLite, providers, tools, and
  telemetry.
- Add incident response, on-call, escalation, and post-incident procedures.
- Add load, soak, stress, and failure-injection environments.

## Reliability and throughput

- Establish target throughput and p95/p99 latency for:
  - API requests,
  - specification reads/writes,
  - execution admission,
  - evidence ingestion,
  - evaluation cases,
  - release resolution.
- Measure bottlenecks before optimizing.
- Use prepared statements and indexed access for SQLite.
- Keep network calls outside database transactions.
- Use bounded concurrency and backpressure everywhere.
- Separate interactive, evaluation, mutation, and telemetry workloads.
- Add horizontal scaling seams behind ports even if the first deployment is
  single-node SQLite.
- Run soak tests long enough to expose leaks, queue growth, and file-handle
  exhaustion.

## Testing

- Unit, integration, contract, E2E, and accessibility gates in CI.
- Load tests with realistic tenant, agent, execution, and evidence distributions.
- Stress tests beyond expected capacity to verify graceful degradation.
- Soak tests for memory, queue, database, CAS, and telemetry stability.
- Chaos tests for provider outage, database restart, worker crash, telemetry
  outage, partial tool failure, and network latency.
- Backup/restore drills with integrity verification.
- Security tests for authentication bypass, tenant escape, SSRF, injection,
  secret leakage, replay, privilege escalation, and denial of service.
- Canary deployment tests with automatic rollback.
- Release-readiness review with explicit accepted risks.

## Deliverables

- SLO and capacity document.
- Production deployment configuration.
- Backup, restore, migration, and rollback runbooks.
- Security and threat-model review.
- Load, soak, stress, and chaos reports.
- On-call and incident-response documentation.

## Exit criteria

- Production failure modes are known and recoverable.
- Capacity limits and scaling thresholds are documented.
- Security controls are tested, not merely configured.
- A deployment can be promoted, monitored, and rolled back safely.
- The platform meets agreed reliability and throughput targets.

