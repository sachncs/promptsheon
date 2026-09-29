# Phase 4 — Evaluation and regression gates

## Objective

Convert execution evidence into repeatable quality, safety, cost, and
performance decisions.

## Responsible

- Evaluation/backend: suite model, runners, scorers, comparisons.
- Domain/data: dataset versioning and case lifecycle.
- Runtime/platform: parallel evaluation execution and capacity.
- Security/safety: policy and adversarial evaluators.
- Frontend/DX: reports, API, SDK, and CLI workflows.

## Implementation

- Define immutable evaluation suites, datasets, cases, scorer versions, and
  run configurations.
- Support deterministic scorers: exact match, regex, schema, safety, latency,
  cost, and tool-use rules.
- Add model-based evaluators with explicit evaluator model and prompt versions.
- Associate every result with agent hash, dataset hash, scorer hash, and run ID.
- Add baseline comparison and regression thresholds.
- Support pass/fail, weighted score, confidence, and statistical summaries.
- Add evaluation policies to agent specifications.
- Expose evaluation reports through API, SDK, CLI, and UI.
- Make failed evaluations explain which cases, rules, or dimensions failed.

## Reliability and throughput

- Schedule independent cases with bounded parallelism.
- Enforce per-suite and per-tenant budgets.
- Cache deterministic results by input, agent hash, evaluator hash, and config.
- Retry transient provider failures but never hide evaluator failures.
- Separate evaluation workloads from production execution capacity.
- Use incremental evaluation for small changes and full suites for promotion.
- Persist partial results so interrupted suites can resume.

## Testing

- Scorer unit tests with boundary and adversarial fixtures.
- Golden evaluation fixtures for stable expected results.
- Dataset version and immutability tests.
- Statistical comparison tests with known distributions.
- Evaluator failure and timeout tests.
- Resume and partial-result recovery tests.
- Contract tests for promotion-blocking behaviour.
- Load tests for concurrent suites, cache hit rate, and provider limits.

## Deliverables

- Evaluation suite and dataset models.
- Scorer registry and versioning.
- Evaluation runner and report model.
- Regression and promotion-gate policies.
- Evaluation dashboard and CLI/SDK workflows.

## Exit criteria

- Candidates can be compared against a repeatable baseline.
- Promotion can be blocked by quality, safety, cost, or latency regression.
- Evaluation runs are resumable, attributable, and reproducible.

