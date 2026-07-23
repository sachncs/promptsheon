# Harness Engineering

Promptsheon's headline surface is the
[harness-engineering](https://openai.com/index/harness-engineering/)
loop: Datasets (ground-truth `{inputs, expected}` pairs),
Preconditions (named command hooks), and Eval Runs (recorded
scoring of a Release against a Dataset). Activate runs every
Capability's enabled Preconditions; a failing Precondition
returns 409 and leaves the Release in `pending`. Eval Runs
return 200 (passed) or 422 (failed) with per-case outcomes
persisted.

The harness runner is **gated** behind
`PROMPTSHEON_HARNESS_PRECONDITIONS=true`. Default is off so
unconfigured deployments don't accidentally execute hooks.
Set the env var in production after preconditions are
audited.

## Datasets

A Dataset is a named collection of `(inputs, expected)`
test cases. The ground truth for harness eval.

```bash
# Create a Dataset from a JSON file.
promptsheon dataset create c1 greeting cases.json
# Or inline:
curl -X POST http://localhost:8080/api/v1/capabilities/c1/datasets \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "greeting",
    "description": "Polite greeting smoke tests",
    "cases": [
      {"seq": 0, "inputs": {"name": "world"}, "expected": "Hello, world!"},
      {"seq": 1, "inputs": {"name": "alice"}, "expected": "Hello, alice!"}
    ]
  }'

# Replace the cases atomically.
curl -X PUT http://localhost:8080/api/v1/datasets/<id>/cases \
  -H 'Content-Type: application/json' \
  -d @cases-v2.json

# Inspect.
curl http://localhost:8080/api/v1/datasets/<id> | jq .
```

## Preconditions

A Precondition is a named command hook on a Capability.
Activate runs every enabled Precondition before transitioning
the Release. A failing Precondition returns 409 and leaves
the Release in `pending`.

```bash
# Add a Precondition.
curl -X POST http://localhost:8080/api/v1/capabilities/c1/preconditions \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "go-test",
    "command": "go test ./...",
    "timeout_sec": 60,
    "enabled": true
  }'

# Update a Precondition (partial).
curl -X PUT http://localhost:8080/api/v1/preconditions/<id> \
  -H 'Content-Type: application/json' \
  -d '{"command": "go test -race ./...", "timeout_sec": 120}'

# Delete a Precondition.
curl -X DELETE http://localhost:8080/api/v1/preconditions/<id>
```

Precondition execution semantics:

- The command runs in the daemon's working directory.
- The environment is scrubbed to a `PROMPTSHEON_*` allowlist
  before exec so a precondition cannot read secrets from
  the daemon's process env.
- The process runs in its own process group; on timeout
  the daemon kills the entire group so a forked child
  cannot outlive the cancellation.
- The `Enabled` flag allows operators to keep a
  Precondition defined but inactive.

## Eval Runs

An Eval Run is a recorded scoring of a Release against a
Dataset using a chosen Scorer.

```bash
# Run an Eval.
curl -X POST http://localhost:8080/api/v1/releases/<id>/evals \
  -H 'Content-Type: application/json' \
  -d '{"dataset_id": "<id>", "scorer": "exact_match"}'

# Inspect.
curl http://localhost:8080/api/v1/evals/<id> | jq .
```

The response is 200 when the run passes (all cases match
the scorer) and 422 when any case fails. Per-case results
are persisted alongside the aggregate.

## Scorers

v0.1.x ships four scorers:

| Scorer | Behaviour |
|--------|-----------|
| `exact_match` | The model's `output` exactly equals the case's `expected`. |
| `contains` | The model's `output` contains the case's `expected` as a substring. |
| `regex` | The model's `output` matches the case's `expected` regex. |
| `json_schema` | The model's `output` is valid JSON that conforms to the case's `expected` JSON Schema document. |

The `json_schema` scorer uses an allow-list of JSON Schema
keywords (SEC-3); unsupported keywords cause a
schema-rejection error.

## Operator guide

1. Write a Dataset of ground-truth cases.
2. Add a Precondition that gates Activate.
3. Drive the iteration loop: create a Version, drive a
   Release lifecycle, run an Eval, look at per-case results.
4. Promote the Version that scores well to production
   (Create + Vote + Activate against a `prod` environment).

Eval runs emit
`promptsheon_eval_cases_passed_total` and
`promptsheon_eval_cases_failed_total`; the SLO alert in
`deploy/prometheus/promptsheon-alerts.yaml` fires when the
failure rate exceeds 10% over 30 minutes.