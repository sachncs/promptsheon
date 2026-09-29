# Phase 5 — Controlled mutation and learning

## Objective

Use evidence and evaluation results to propose better agent configurations
without allowing uncontrolled self-modification.

## Responsible

- Agent systems/backend: mutation strategies and candidate generation.
- Evaluation/backend: candidate validation and comparison.
- Security/governance: risk classification and allowed mutation scope.
- Product/frontend: proposal review and explanation.
- SRE/platform: mutation workload isolation and budgets.

## Implementation

- Define immutable `MutationProposal` records containing source hash, proposed
  changes, rationale, expected outcome, author/system, risk, and confidence.
- Implement bounded mutation types:
  prompt, guardrail, model, routing, context, tool, permission, execution,
  memory, and budget policy changes.
- Build deterministic rule-based mutation strategies first.
- Add evaluator-guided search for candidate changes.
- Add LLM-assisted proposal generation only behind explicit policy.
- Validate proposals against schema, security policy, budget, and allowed fields.
- Materialize every candidate as a new agent hash.
- Automatically execute required evaluation suites.
- Compare candidate against current baseline and historical best.
- Require explicit approval for high-risk changes.
- Record why a candidate was accepted, rejected, or abandoned.

## Reliability and throughput

- Isolate mutation jobs from production execution workers.
- Limit candidate fan-out, recursion depth, and total search budget.
- Deduplicate equivalent candidates by content hash.
- Stop search early when quality, safety, or cost thresholds cannot be met.
- Use priority queues for user-requested proposals over background exploration.
- Preserve all intermediate candidates for audit, but apply retention policies.
- Prevent mutation loops by tracking ancestry and rejected hashes.

## Testing

- Mutation schema and allowed-field tests.
- Property tests proving mutations never alter the source revision.
- Security tests for permission escalation and guardrail weakening.
- Candidate deduplication and ancestry tests.
- Evaluation-gate tests for accepted and rejected candidates.
- Budget and search-termination tests.
- Replay tests proving the same proposal produces the same candidate when
  deterministic strategy inputs are used.
- Adversarial tests for prompt injection through evidence or evaluator output.

## Deliverables

- Mutation proposal model.
- Candidate generation and validation service.
- Candidate comparison workflow.
- Risk and approval policies.
- Mutation audit and operator controls.

## Exit criteria

- The platform can propose improvements but cannot silently activate them.
- Every candidate is immutable, evaluated, risk-classified, and explainable.
- Mutation workloads are bounded and isolated from production traffic.

