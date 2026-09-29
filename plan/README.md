# Promptsheon implementation plan

This plan turns Promptsheon from an agent runtime with supporting tooling into
an evidence-driven platform for engineering and evolving agentic intelligence.

The plan is organised around one production vertical slice:

```text
Define an agent
  → persist an immutable revision
  → execute it
  → collect evidence
  → evaluate it
  → propose a mutation
  → validate the candidate
  → approve and promote it
  → observe and roll back it
```

## Product invariant

Production must always execute an immutable, content-addressed
`AgentSpecification`. Every execution, evaluation, mutation, approval, and
release must reference the exact specification hash that was involved.

## Phase map

| Phase | Outcome | Primary owner |
|---|---|---|
| 0 | Reliable baseline and supported product surface | Engineering lead |
| 1 | Immutable agent specification and lineage | Domain/backend |
| 2 | Durable execution engine | Runtime/backend |
| 3 | Evidence, tracing, and observability | Platform/operations |
| 4 | Evaluation and regression gates | Evaluation/backend |
| 5 | Controlled mutation and learning proposals | Evaluation/agent systems |
| 6 | Approval, promotion, canary, and rollback | Release/platform |
| 7 | Coherent product UX and developer experience | Product/frontend |
| 8 | Executable developer and operator documentation | Developer experience |
| 9 | Production hardening and throughput validation | SRE/security |

Phases are ordered by dependency, but implementation should use small vertical
slices. Do not build autonomous learning before immutable specifications,
evidence, evaluation, and promotion gates exist.

## Definition of done for every phase

- Implementation is inside the correct package boundary.
- Public contracts are validated and documented.
- Unit, integration, and failure-path tests exist at the relevant layer.
- Reliability and throughput impact is measured, not assumed.
- Migrations and rollback procedures are documented.
- Telemetry identifies failures, latency, saturation, and cost.
- CI remains green after every logical change.
- The phase exit criteria are met before the next phase becomes critical-path work.

## Ownership model

- **Engineering lead:** scope, architecture, sequencing, acceptance criteria.
- **Domain/backend:** schemas, use cases, repositories, migrations, API contracts.
- **Runtime/backend:** execution lifecycle, providers, tools, cancellation, budgets.
- **Frontend/product:** workflows, information architecture, accessibility, visual QA.
- **Platform/SRE:** deployment, observability, resilience, capacity, operations.
- **Security:** threat model, authorization, secrets, isolation, auditability.
- **Developer experience:** SDK, CLI, examples, documentation, release notes.

Each pull request must name the responsible owner and include the tests and
operational evidence required by the phase it advances.

