# Harness engineering

Promptsheon's headline surface for the harness engineering loop
that the [OpenAI harness engineering article](https://openai.com/index/harness-engineering/)
describes. Three primitives compose:

| Primitive | Role |
|---|---|
| **Dataset** | Ground truth: `{inputs, expected}` pairs that the LLM must satisfy. |
| **Precondition** | A named command hook run on every Activate. Failing hooks block the release. |
| **Eval** | A recorded scoring run of a Release against a Dataset. Score is `passed / total`. |

The daemon wires all three into the existing release lifecycle.
A Release that fails its preconditions returns `409 Precondition
Failed`; an Eval Run that fails its scorer returns `422 Unprocessable
Entity`.

## Datasets

A Dataset is a collection of test cases attached to a Capability.
Stored as JSON; the LLM output for each case is compared against
the `expected` value using the Scorer selected at run time.

```bash
# Create from a JSON file
promptsheon dataset create c1 --name greeting --file cases.json

# cases.json shape (array form):
# [
#   {"inputs": "hi",  "expected": "hi"},
#   {"inputs": "bye", "expected": "bye"}
# ]

# Or wrapped form:
# {"cases": [{"inputs": "hi", "expected": "hi"}]}

# List, get, replace cases, delete
promptsheon dataset list c1
promptsheon dataset get <dataset_id>
promptsheon dataset put-cases <dataset_id> cases.json
promptsheon dataset delete <dataset_id>
```

## Preconditions

A Precondition is a named shell command attached to a Capability.
The daemon runs every enabled precondition when a Release is
activated; a non-zero exit blocks the release and surfaces the
captured output to the operator.

```bash
promptsheon precondition add c1 --name go-test --cmd "go test ./..." --timeout 60
promptsheon precondition list c1
promptsheon precondition delete <id>
```

A 409 response on Activate has the shape:

```json
{
  "error": "harness: precondition failed: go-test",
  "failures": [
    {"name": "go-test", "output": "FAIL\t...\n"}
  ]
}
```

## Evals

An Eval Run invokes the Release once per Dataset case and scores
the actual output against `expected`. The Run persists per-case
outcomes and an aggregate `score = passed / total`. Built-in
scorers today:

| Scorer | Behaviour |
|---|---|
| `exact_match` | byte-equal after JSON canonicalisation |
| `contains` | substring match (case-sensitive) |
| `regex` | Go regex; `expected` is the pattern, `actual` is matched |
| `json_schema` | placeholder (M3 follow-on) |

```bash
promptsheon eval run <release_id> --dataset <dataset_id> [--scorer exact_match]
promptsheon eval list <release_id>
promptsheon eval get <eval_id>
```

A failing run returns `422`; the response body is the `EvalRun`
with `Status = "failed"` and `Score = passed/total`. Callers drive
their own UI off `Passed`, `Failed`, and `Score`.

## The iteration loop

The fast feedback loop the harness engineering article prizes:

```text
1. write dataset + add preconditions
2. (loop)
   a. write a new Version
   b. CreateRelease + Vote + Activate
      → Activate runs preconditions; 409 if any fail
   c. EvalRun against the Release
      → 422 if any case fails the scorer
   d. read the per-case EvalResult; iterate
3. promote the version that passes
```

This is the same loop as for traditional unit tests: write a test,
run it, see red, fix, run again. The Score column in the EvalRun
is the green/red signal.

## Programmatic surface

The same surfaces are exposed via the Go SDK:

```go
ds, _ := client.CreateDataset(ctx, "c1", sdk.CreateDatasetRequest{
    Name: "greeting",
})
client.PutCases(ctx, ds.ID, []sdk.DatasetCase{
    {Seq: 0, Inputs: json.RawMessage(`"hi"`), Expected: json.RawMessage(`"hi"`)},
})

p, _ := client.CreatePrecondition(ctx, "c1", sdk.CreatePreconditionRequest{
    Name: "go-test", Command: "go test ./...", TimeoutSec: 60,
})

run, _ := client.RunEval(ctx, "<release_id>", sdk.RunEvalRequest{
    DatasetID: ds.ID, Scorer: "exact_match",
})
fmt.Printf("score=%v status=%s\n", run.Score, run.Status)
```

And via the REST API:

```
POST   /api/v1/capabilities/{capability_id}/datasets
GET    /api/v1/capabilities/{capability_id}/datasets
GET    /api/v1/datasets/{id}
PUT    /api/v1/datasets/{id}/cases
DELETE /api/v1/datasets/{id}

POST   /api/v1/capabilities/{capability_id}/preconditions
GET    /api/v1/capabilities/{capability_id}/preconditions
DELETE /api/v1/preconditions/{id}

POST   /api/v1/releases/{release_id}/evals
GET    /api/v1/releases/{release_id}/evals
GET    /api/v1/evals/{id}
```

See [api-reference.md](api-reference.md) for full request/response
shapes.

## What this is not

- Not a training pipeline. Promptsheon does evals at deploy time,
  not during model fine-tuning.
- Not an LLM-as-judge scorer (yet). The `json_schema` scorer is
  a placeholder; an LLM-judge variant lands in a follow-on commit.
- Not a parallel eval orchestrator. Cases run serially today;
  parallel runners ship when the harness eval corpus gets large
  enough to warrant the complexity.
