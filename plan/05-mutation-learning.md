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

## Implementation status

The first governed slice is now implemented: mutation proposals are stored as
tenant-scoped records with immutable source references, explicit mutation kinds,
structured changes, risk, confidence, author, and evaluation linkage. The API
supports listing, inspection, and guarded approve/reject/abandon decisions.
The console exposes the review queue and requires an auditable decision reason.
The local simulator can author deterministic proposals without an LLM key.
The goal evolution endpoint now runs in proposal mode: it writes the candidate
to content-addressed storage, records the proposal, and leaves the active
manifest unchanged until an explicit decision.

Candidate manifests now have an explicit credential-free validation step. The
validation service integrity-checks the CAS object, parses the manifest schema,
requires `metadata.capabilityId`, and moves a proposal from `proposed` to
`validated`. Approval is only accepted from `validated` proposals, and the
console exposes the validation action before approval.

Proposal-mode evolution now executes and scores the candidate manifest before
persisting the proposal. The proposal records baseline and candidate scores in
its structured changes, and links the candidate trace when observability is
enabled. Evaluation passes only when the candidate reaches the manifest
threshold and does not regress against the measured baseline. The active
manifest is still left unchanged. Approval additionally requires a durable
evaluation-run reference, and the console keeps approval disabled until that
evidence is present. Reviewers can attach a completed, passing,
organisation-scoped evaluation-suite run to a materialised proposal; the
candidate score and suite threshold are read from the durable run rather than
accepted from the browser. A failed proposal evaluation can be replaced with
a new durable run from the console while the backend continues to enforce the
same candidate, tenant, completion, and threshold checks.

Automatic candidate materialisation and evidence-backed promotion remain in
progress. Declared dataset cases now run through the graph executor and the
configured evaluator with bounded fan-out; missing, empty, malformed, or
oversized datasets fail closed. The approval ledger is not
connected to automatic activation: an approved proposal can be explicitly
materialised as a draft release, while activation remains governed by the
existing release approval and lifecycle gates.

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
