# ADR 0011: Release and Approval are separate aggregates

- **Status:** Accepted
- **Date:** 2026-07-10
- **Supersedes:** —
- **Superseded by:** —

## Context

The current model treats `Deployment` as the operational pointer to a Capability Version in an Environment. Conceptually, three distinct lifecycle events are conflated into one type:

1. **Approval** — a human or policy decided this Version is fit for an Environment.
2. **Release** — that decision is now bound to a Version and an Environment, awaiting traffic.
3. **Deployment** — traffic is actually flowing.

Conflating these means changing the quorum rule rewrites Release history; recording multiple approvers mixes audit signals; rollback conflates "we changed our mind" with "traffic shifted".

The Project Philosophy (charter) lists Clear Ownership and Event Driven as engineering principles. Conflating the three concepts violates both.

## Decision

Split the lifecycle into three aggregates, each with one responsibility:

```
internal/release     — Release: approved pointer from Version to Environment
internal/approval    — Approval: votes, quorum, separation-of-duties
internal/capability  — Deployment: traffic pointer (existing aggregate)
```

A `Release` is constructed `Pending`, moves to `Approved` when `Approval.Policy.Evaluate` returns `Approved`, and `Activate`s after the corresponding `Deployment` reports success. `Rollback` produces a successor `Release` that re-points the Environment; the original `Release` becomes `Superseded` or `RolledBack`.

`Approval.Policy` is an interface. Two implementations ship in v0:

- `MajorityPolicy{Required int}` — absolute vote count.
- `MakerCheckerPolicy{RequiredApprovers int}` — paired with `VerifySeparationOfDuties` to enforce that the creator is not in the Approver list.

Policies live in the `approval` package rather than in `release` so quorum rules can evolve without rewriting Release history.

`Release` is value-immutable. Transition methods return new values.

## Options considered

1. **Three aggregates with one responsibility each (chosen).** Each aggregate owns one invariant; clear audit; quorum rules are data, not code paths.
2. **Single Deployment aggregate with extra fields.** Status quo. **Rejected** — every audit report must mix policy and traffic.
3. **Two aggregates (Release + Approval merged).** Marginal saving at the cost of policy evolution.

## Consequences

Positive:

- Approval audit is queryable independently of Release traffic.
- Quorum policy changes don't rewrite Release history.
- The same `Approval` mechanism governs optimizer-adopted Versions, manual changes, and emergency rollbacks.
- `Rollback` is now a non-destructive transition that produces a successor Release, so the predecessor is preserved for audit.

Negative:

- Three aggregates to reason about instead of one. The trade is worth it because each has a single invariant and clear ownership.
- The Repository layer in `internal/store/` will eventually need three corresponding tables (`releases`, `approvals`, `deployments`) that may share a `release_id` foreign key. This is a migration-heavy change that proceeds release-by-release.

## References

- `internal/release/release.go` — Release aggregate
- `internal/release/release_test.go` — lifecycle transitions
- `internal/approval/approval.go` — Approval aggregate and policies
- `internal/approval/approval_test.go` — quorum and maker-checker tests
- `internal/capability/deployment.go` — existing Deployment type
