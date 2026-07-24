# Eval Primitive

The Eval primitive is the workhorse of the harness
engineering loop. An Eval Run is a recorded scoring of a
Release against a Dataset using a chosen Scorer.

## Lifecycle

```
   create
    │
    ▼
  running  ─── scorer fails ───▶ error
    │
    ▼
  passed (no case failed)  OR  failed (any case failed)
```

Status values:

| Status | When |
|--------|------|
| `running` | Eval Run created; cases being invoked. |
| `passed` | All cases passed the scorer. |
| `failed` | At least one case failed. |
| `error` | The runner itself errored (e.g. unknown scorer, dataset missing). |

When a Run finishes (passed/failed/error), the per-case
`EvalResult` rows are persisted and the aggregate counters
increment `promptsheon_eval_cases_passed_total` or
`promptsheon_eval_cases_failed_total` accordingly.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/releases/{release_id}/evals` | Run an Eval. Body: `{dataset_id, scorer}`. |
| `GET`  | `/api/v1/releases/{release_id}/evals` | List EvalRuns for a Release. |
| `GET`  | `/api/v1/evals/{id}` | Get an EvalRun with per-case results. |

## Run a Scorer

`runCase(ctx, run, scorer, case)`:

1. Decode `case.Inputs` (JSON) into a `map[string]any`.
2. Call `r.Inv.Invoke(ctx, run.ReleaseID, inputs)`.
3. Score `actual` against `case.Expected` using the Scorer.
4. Record the result; return.

Failures (scorer error or LLM call error) are recorded on
the case's `EvalResult.Error` field and the case is marked
`Passed: false`.

## Scorers

v0.2.0 ships four scorers. Each is registered in
`internal/eval` and discoverable via `eval.ValidScorers`:

| Scorer | Behaviour |
|--------|-----------|
| `exact_match` | `actual == expected` (byte-equal). |
| `contains` | `strings.Contains(actual, expected)`. |
| `regex` | `regexp.MatchString(expected, actual)`. |
| `json_schema` | `expected` is a JSON Schema document; `actual` must be a valid JSON value that conforms to the schema. |

The `json_schema` scorer is gated by an allow-list of
JSON Schema keywords (SEC-3). Unsupported keywords cause
the schema to be rejected at validation time, not silently
ignored.

## Case record

`EvalResult`:

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Server-generated unique ID. |
| `run_id` | string | The parent `EvalRun.ID`. |
| `case_id` | string | The `DatasetCase.ID` that produced this result. |
| `seq` | int | The case's position in the dataset. |
| `passed` | bool | Whether the scorer accepted the output. |
| `actual` | json.RawMessage | The model's actual output. |
| `error` | string | Empty on pass; populated on scorer or invoke failure. |
| `latency_ms` | int64 | Wall-clock per case. |

## Aggregate record

`EvalRun`:

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Server-generated. |
| `release_id` | string | The Release that was evaluated. |
| `dataset_id` | string | The Dataset the cases came from. |
| `scorer` | string | The Scorer name (e.g. `exact_match`). |
| `score` | float64 | `passed / total` if `total > 0`, else `0`. |
| `passed`, `failed`, `total` | int | Per-case counts. |
| `status` | string | `running` / `passed` / `failed` / `error`. |
| `started_at`, `finished_at` | timestamp | Wall-clock. |

## Serial execution

v0.2.0 runs cases serially. Each case invokes the Release
through the configured LLM provider; the next case doesn't
start until the previous finishes. Parallel execution
ships in a follow-on.

## SLOs

`promptsheon_eval_cases_passed_total` and
`promptsheon_eval_cases_failed_total` are the metrics the
SLO alert in
`deploy/prometheus/promptsheon-alerts.yaml` queries. The
alert fires when the failure rate exceeds 10% over 30
minutes.

## SDK

```go
run, err := client.RunEval(ctx, rel.ID, sdk.RunEvalRequest{
    DatasetID: datasetID,
    Scorer:    "exact_match",
})
```

`run.Status` is one of `"running"`, `"passed"`, `"failed"`,
`"error"`. The per-case results are available via
`client.GetEval(ctx, run.ID)`.

## CLI

```bash
promptsheon eval run <release_id> <dataset_id> [scorer]
promptsheon eval list <release_id>
promptsheon eval get <id>
```

## See also

- [docs/harness.md](harness.md) — Datasets, Preconditions,
  Eval Runs as a single loop.
- [docs/observability.md](observability.md) — the metric
  inventory.