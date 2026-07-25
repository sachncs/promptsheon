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

v0.2.0 ships four scorers:

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
## Closed-loop self-evolution

The harness exposes a closed loop where the daemon revises a
Capability's prompt artifact when the active Release's
EvalRun score drops below a threshold. The loop is the
product's headline feature: the daemon finds a real bug in
its own prompt, generates a fix, validates the fix, and
promotes it — all without a human in the loop, all behind
a cap.

### Configuration

Per-Capability, opt-in via the `self_evolve_*` columns on
`capabilities` (migration `019_self_evolve`). The boot
path also accepts `PROMPTSHEON_SELF_EVOLVE` to seed the
config at startup:

```
PROMPTSHEON_SELF_EVOLVE="cap_id:dataset_id:threshold:target_env:max_revisions:cooldown_sec;..."
```

`PROMPTSHEON_SELF_EVOLVE_MODEL` overrides the LLM model
the revision and validation calls use (default `MiniMax-M2.7`).

| Field | Meaning | Default |
|-------|---------|---------|
| `enabled` | Master switch | false |
| `min_score` | Promote only if validation score ≥ this | 0.9 |
| `max_revisions` | Hard cap on revisions per cycle | 10 |
| `cooldown_sec` | Minimum gap between cycles | 900 (15 min) |
| `target_env` | Auto-promote env (typically `dev`) | dev |
| `dataset_id` | Dataset to validate candidate against | (required) |

### The cycle

When `ContinuousEval`'s most recent EvalRun for the
active Release scores below `min_score` and the cooldown
has elapsed, the evolver:

1. **Detect** — writes a `self_evolve.detect` audit row.
2. **Read failing cases** — pulls per-case outcomes from
   the most recent EvalRun and the dataset to build the
   revision payload.
3. **Revise** (loop up to `max_revisions`) — invokes a
   revision LLM (`makeEvolverLLMInvoke` in
   `cmd/promptsheond/evolver_wire.go`) with the current
   prompt, the failing cases, and the seeded
   `DefaultRevisionLLMSystem` instruction. The LLM returns
   a new prompt text. Writes a `self_evolve.revise` row
   per attempt.
4. **Validate** — runs the new prompt through the
   candidate's dataset via a direct LLM call (no DB writes),
   scores with the chosen Scorer, returns a synthetic
   `EvalRun`. Writes a `self_evolve.validate` row.
5. **Promote** — on validation success, writes the new
   prompt to CAS (`pkg/cas.WriteObject`), creates a new
   `capability.Version` reusing the active Release's
   `model_policy` and `runtime_policy` hashes, creates a
   `release` row in the target env, and `Service.SelfActivate`s
   it via a `SelfApprovePolicy` (bypasses maker-checker
   for self-evolved Releases only). Writes a
   `self_evolve.promote` row.
6. **Reject** — if `max_revisions` is hit without
   validation success, writes a `self_evolve.reject` row
   and stamps the cooldown. The active Release is
   unchanged.

### Safety rails

- `max_revisions=10` per cycle caps the LLM spend.
- `cooldown_sec` between cycles prevents thrash.
- `target_env=dev` only — `staging` and `prod` are never
  touched by the evolver.
- The `SelfApprovePolicy` is only used by `Service.SelfActivate`;
  regular `Service.Activate` (human-initiated) still runs
  the configured `maker-checker` / `majority` policy.
- Every state change is in the audit chain; the evolver
  cannot silently mutate a Release.
- The `Capability.SelfEvolve` config is opt-in; a Capability
  with `enabled=false` is never touched.

### Observability

- Audit chain: `self_evolve.{detect,revise,validate,promote,reject,skip}`.
- `self_evolve_state` table: persisted cooldown + cycle
  state, survives daemon restart.
- Daemon log: each cycle logs `RunOnce result` with
  `promoted / skipped / revisions / score / duration_ms / reject_reason`.

### Limitations

- Self-evolve revises **prompt artifacts only**. It does
  not modify Go source, model policies, or runtime policies.
- The validation regex/scorer must match what the LLM
  can actually produce. If the dataset's expected values
  are unreachable for the model (e.g., a regex the model
  can't satisfy), the cycle will reject after
  `max_revisions`.
- The revision LLM and the validated LLM are the same
  model (configured via `PROMPTSHEON_SELF_EVOLVE_MODEL`).
  Operators that want a stronger revision model than
  the validated model can run a smaller downstream model.
- Self-evolve is a background loop; it does not run on
  the eval path. The `ContinuousEval` is the trigger.

### Quick start

The fastest path to a working self-evolve loop:

```bash
# 1. Pick a capability and its eval dataset.
CAP=...   # id
DS=...    # dataset id

# 2. Set PROMPTSHEON_SELF_EVOLVE in the daemon env.
# Format: cap_id:dataset_id:threshold:target_env:max_revisions:cooldown_sec
export PROMPTSHEON_SELF_EVOLVE="$CAP:$DS:0.9:dev:10:900"
# Optional: override the model the revision LLM uses.
export PROMPTSHEON_SELF_EVOLVE_MODEL="MiniMax-M2.7"

# 3. Boot the daemon. The evolver loop starts automatically
#    for any capability with the matching env entry.
./promptsheond
```

The CLI also exposes the same surface for live toggling:

```bash
# Enable (idempotent; merges with existing config).
./promptsheon selfevolve enable $CAP \
  --dataset $DS \
  --min-score 0.9 \
  --max-revisions 10 \
  --cooldown-sec 900 \
  --target-env dev

# Inspect persisted config.
./promptsheon selfevolve status $CAP

# Disable (other fields preserved).
./promptsheon selfevolve disable $CAP
```

The daemon exposes self-evolve counters on the standard
`/metrics` endpoint:

```
promptsheon_self_evolve_runs_total        # total RunOnce ticks
promptsheon_self_evolve_revisions_total   # total revision attempts
promptsheon_self_evolve_promoted_total    # total successful promotes
```

For an end-to-end smoke test (boot, seed, watch the
cycle complete, assert audit + metrics):

```bash
./scripts/selftest.sh
```
