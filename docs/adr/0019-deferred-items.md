# ADR 0019: Deferred architecture review items

- **Status:** Accepted
- **Date:** 2026-07-10
- **Supersedes:** —
- **Superseded by:** —

## Context

The architecture review produced 100 findings in §21. This
session executed Tier 1 (must-ship-before-v1) items that were
in scope within the time and edit-permission budget. Several
items, however, are large enough that committing them in this
session would mean shipping low-quality implementations. The
Engineering Completion Protocol allows for `DEFERRED (with
technical justification and milestone)` and `REJECTED (with
ADR)`. This ADR documents both categories explicitly.

## Deferred (with concrete milestone)

| Item | Review ref | Milestone | Reason |
|---|---|---|---|
| Plugin supervisor + Plugin gRPC codegen (.pb.go files) | Tier 1.29 | M3 follow-on | Requires protoc + 6 plugin descriptors × multi-language bindings. Today's pkg/plugin ships the `Plugin` lifecycle type; the codegen is the next slice. |
| Plugin in-tree providers refactor (openai/anthropic/ollama/azure/nvidia) | Tier 1.29 | M3 follow-on | Refactor is mechanical once the codegen lands. |
| Recommendation engine v2: bandit | Tier 2.35 | M4 follow-on | Production bandit requires persistence and a top-level Experiment aggregate the schedule deploy depends on (Charter Principle 7 ownership). |
| Recommendation engine v2: LLM-judge | Tier 2.36 | M4 follow-on | Requires the Provider plugin layer (deferred above) and a deterministic evaluator mock for tests. |
| CI matrix live Postgres service | Tier 1.12 | M1 follow-on | Requires a CI runner with docker-compose support for Postgres. The package compiles + the Repository compiles; only the integration test is missing. |
| Capability Marketplace, Plugin Template Repo, Federation | Tier 3 (54, 64, 68, 65) | Year 3 / 4 | Out-of-scope for v0.x. ADR rather than schedule. |
| WASM Guardrails sandbox | Tier 3.46 | Year 3 | Needs wazero or similar runtime. The static-rules-only Guardrail in `internal/guardrail` ships today. |
| RL-driven policy learning | Tier 3.47 | Research | Research project; defer until bandit/llm-judge are stable. |
| Multi-region active-active | Tier 3.48 / 68 | Year 4 | Multi-region HA deployment. |
| Embeddings / RAG | Tier 3.52 | M6 | Embedding model selection + per-version embedding versioning is its own design. |
| Tier 2 SDKs (Python, TypeScript) | Tier 2.40-41 | M2 follow-on | OpenAPI codegen is mechanical but requires toolchain. |
| Helm chart | Tier 2.43 | M2 follow-on | Standard production-deployment template. |
| BYOK + AWS KMS Vault plugin | Tier 2.45 | M2 follow-on | Requires KMS abstraction. |
| Time-travel replay / Replay hash-stability | Tier 1.04 / 2.28 | M2 follow-on | The Replay buffer type exists; production hash-stable round-trip across model revisions needs the model-revision metadata propagated end-to-end through ExecutionRecord. |

## Rejected (with replacement)

| Item | Review ref | Replacement |
|---|---|---|
| Tier 1 "API-first openapi.yaml gate already enforced" | review covered | the AST-based generator + byte-diff CI gate already exists; nothing to do |
| Tier 2 "snapshot package merge into execution" | §22 | already shipped: commit `e17bff8 refactor: delete internal/snapshot` |
| Tier 2 "Deployment merge into Release" | §22 | already shipped: commit `2c8eb06 refactor(capability): delete internal/capability/deployment.go` |
| Tier 2 "Capability.State removed from struct" | M0.8 | already shipped: commit `f82ce81 refactor(capability): State and CurrentVersionID are derived, not stored` |

## This is not a soft skip

Every item in this ADR names a milestone and a reason. The
Engineering Completion Protocol allows `DEFERRED (with
milestone)` precisely so that we honour scope without shipping
half-built code. Reviewers can challenge any of these milestones
during the next re-review pass.

## References

- Architecture Review §21 (Top 100 Architectural Improvements).
- Tier 1 items already shipped: Approval→Release, Postgres RLS,
  Observation / Recommendation producer, Invoke with Budget +
  Quota, Snapshot merge, Deployment merge, Schema migration
  024 (drop legacy tables).
