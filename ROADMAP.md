# Roadmap

Promptsheon is the AI Capability Control Plane — the
operating system for versioning, deploying, governing,
observing, and optimizing AI Capabilities at enterprise scale.

This document tracks the explicit forward-only plan for the
next three releases.

## v0.3.0 (current)

The headline: the recommendation loop is closed in
production, Capability Contracts + Diff + Reputation + Catalog
ship, the release lifecycle is TLA+-specified, and the SDKs
match the server.

**Status**: in development. 46 atomic items landed across
Day 1/2/3 closure pass. See CHANGELOG.md for the full list.

**Acceptance**: `make check && make test && make bench &&
make docs-check && make purity` all pass in CI; the Go SDK
(`pkg/promptsheon`) covers every `/api/v1` route in the
`promptsheon/spec/spec.yaml` OpenAPI surface (the parity
gate is mechanical — see PR-5 in
`docs/research/audit-fixes-plan.md`); the TLA+ specs
(audit chain + release lifecycle) have their Go-side
regression tests in place.

**SDK scope change**: the `sdk/python/` and `sdk/typescript/`
directories were removed in v1.0.0. They had only contained
a copy of the OpenAPI spec and no actual client code; the
`make sdk` / `make sdk-check` targets and the corresponding
`sdk-python` / `sdk-typescript` CI jobs were removed at
the same time. Only the Go SDK ships today. A future
generator pass can re-introduce the Python and TypeScript
clients — when it does, the parity gate from PR-5 will
mechanically catch any drift between the spec and the
generated SDK.

## v0.4.0

Multi-region + canary + gRPC + pgx.

- **Multi-region replication.** Per-region audit chain +
  global Merkle-root checkpoint design. CRDT settings,
  bandit, replay, and idempotency caches already designed in
  `docs/research/`; this milestone lands the multi-region
  backend.
- **pgx backend.** v0.3.0 ships the schema, RLS policies, and
  in-memory adapter under `promptsheon/store/postgres/`. v0.4.0
  replaces the in-memory adapter with a real pgx-backed
  implementation and adds the `DatabaseURL` config knob.
- **Canary Release primitive.** `docs/reference/canary.md` already
  exists; v0.4.0 wires the runtime: N% traffic to a new
  Version, atomically superseded on promote.
- **gRPC plugin transport.** The `.proto` file is committed;
  the runtime swap from net/rpc to gRPC is a v0.4.0 deliverable.
- **LLM-judge scorer at scale.** v0.3.0 ships the primitive;
  v0.4.0 ships the production JudgeClient wiring through the
  LLM gateway, with caching, batching, and SLO observability.

## v0.5.0

Capability marketplace + reputation as a market signal.

- **Capability Marketplace.** Signed Manifest packages
  installable cross-Workspace. The signed package format
  mirrors Docker images: name + version + manifest hash +
  signatures. The Inheritance primitive (v0.3.0) is the
  composition unit.
- **Capability Reputation as a market signal.** Trust score
  ranks Capabilities in the marketplace; reputation is
  transportable across Workspaces (a high-reputation
  Capability from one Workspace is a credible install for
  another).
- **Recommendation auto-promotion at scale.** v0.3.0 closes
  the loop and adds the Reasoning Compiler; v0.5.0 tunes
  per-Workspace policy, bandit sample-ratio configuration,
  and SLO-driven thresholds.
- **OpenTelemetry trace export.** v0.2.0 ships OTLP-only;
  v0.5.0 ships the Trace Visualizer.
- **Decision audit replay.** Re-derive the system state from
  the audit chain alone; the harness loop, the bandit, the
  reasoning compiler, and the recommendation producer are
  all re-derivable from audit + observations + decisions.

## v1.0.0

Stable API + the substrate moment.

- **API stability.** No breaking changes from v1.0.0 onward;
  all additions are additive.
- **Capability inheritance + marketplaces are the default
  deployment unit.** New Capabilities are composed, not
  written from scratch.
- **Memory is a tiered fabric.** Sensory → working →
  episodic → semantic → procedural, all governed by
  Capability contracts.
- **Reasoning is compilation.** The LLM is a backend; the
  Capability is the surface.
- **Tools are a Mesh.** Content-addressed, capability-typed,
  MCP-native.
- **Policies are constitutional.** The Capability has a
  written constitution; the Release checks it.
- **Reputation is the unit of trust.** Capabilities have
  scores; marketplaces rank them.
- **CRDTs are the default for distributed state.** No more
  single-leader; no more split-brain.
- **Formal specs are first-class.** TLA+ for ordering,
  property tests for CRDTs, fuzz for security-critical paths.
- **Capabilities are tradeable.** A signed Manifest is a
  market instrument.
- **Reasoning futures markets.** Forward contracts on
  Recommendation outcomes (only if the category stabilises).
- **gRPC streaming for real-time updates.**

## Deferred (post-v1.0)

- LLM-judge ensemble scorers.
- Federation across orgs.
- On-prem appliance form factor.
- Hardware acceleration (TPU/GPU inference scheduling).
- Reasoning compilers (prompt → bytecode → verifiable
  execution).
