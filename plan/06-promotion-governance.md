# Phase 6 — Promotion, lifecycle, and governance

## Objective

Promote validated candidates safely and make rollback deterministic.

## Responsible

- Release/platform: release state machine and promotion orchestration.
- Security/governance: authorization, approvals, signatures, audit.
- Runtime/backend: routing and active-release resolution.
- SRE: canary monitoring, rollback, incident procedures.
- Frontend/DX: approval, release, and rollback workflows.

## Implementation

## Implementation status

Mutation candidates can be explicitly promoted into draft releases after
approval. Release transitions into `approved`, `canary`, or `active` now also
require an organization-scoped passing evaluation run in addition to the
maker-checker approval gate. Draft creation never activates traffic.
Release manifests can now be signed with organization-scoped Ed25519 keys;
promotion verifies the signature against the canonical manifest hash and
rejects missing, tampered, or deactivated-key signatures.

Canary rollout automation is now lifecycle-safe at the release boundary.
Canary traffic rules are limited to routable releases, and promoting a canary
to active atomically retires the existing active peer while resetting the
promoted release's canary weight. Promotion conflicts are surfaced as a
retryable conflict rather than falling back to a non-atomic status update.
Promotion concurrency and restart recovery are covered by durable idempotency
tests. A deterministic canary health assessment now evaluates the latest
organization-scoped run and atomically rolls back a regressed canary/active
release to its active peer. A bounded background monitor now assesses all
canaries every minute, prevents overlapping polls, and stops during shutdown.

- Define lifecycle states: draft, validated, approved, canary, active,
  deprecated, rolled back, and archived.
- Keep release records separate from immutable agent specifications.
- Require evaluation and policy gates before approval.
- Enforce maker-checker separation.
- Add signed release manifests and verification.
- Add environment-specific promotion rules.
- Add percentage, segment, and header-based canary routing.
- Add automated rollback conditions for error, quality, cost, or latency
  regressions.
- Add manual rollback to the last verified hash.
- Add release notes from specification and evidence diffs.
- Record every transition in an append-only audit trail.

## Reliability and throughput

- Resolve the active release through an atomic, cached read path.
- Keep rollout configuration immutable per release revision.
- Make promotion and rollback idempotent.
- Ensure a failed rollout cannot leave mixed or unknown state.
- Bound canary evaluation windows and sample sizes.
- Protect release operations with locks or compare-and-swap semantics.
- Keep audit writes durable without blocking request execution unnecessarily.

## Testing

- State-machine tests for all lifecycle transitions.
- Approval and maker-checker authorization tests.
- Signature verification and tamper tests.
- Canary distribution and routing tests.
- Promotion concurrency and idempotency tests.
- Automated rollback tests using injected regressions.
- Restart tests while promotion is in progress.
- Audit-chain verification tests.
- End-to-end test from candidate evaluation to active release and rollback.

## Deliverables

- Release lifecycle service.
- Approval and policy gates.
- Signed release manifest.
- Canary router and rollback controller.
- Release dashboard and operational runbook.

## Exit criteria

- Production traffic always maps to a known immutable hash.
- No candidate can bypass required evaluation or approval.
- Canary rollout and rollback are tested under failure.
- Operators can prove who approved and promoted every release.
