# Workflows

A Workflow is a sequence of named `Step`s. Each Step is
executed in declaration order; a step failure short-circuits
the remaining steps and the Workflow returns 422. The
Workflow runs in-process (no external executor) and shares
the daemon's context (request ID, user ID, audit chain).

The current Step model is `{ID, Tool, Input, Output}`:

```json
{
  "id": "summarise-then-translate",
  "steps": [
    {
      "id": "summarise",
      "tool": "http",
      "input": {
        "method": "POST",
        "url": "https://summariser.local/run",
        "body": {"text": "{{input.doc}}"}
      }
    },
    {
      "id": "translate",
      "tool": "shell",
      "input": {
        "command": "translate --target {{input.target_lang}} --in /tmp/summary.txt --out /tmp/translated.txt"
      },
      "output": "translated_path"
    }
  ]
}
```

When `output` is set, the step's result is written into the
Workflow's outputs under that key, so a later step's input can
reference `{{outputs.translated_path}}`.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/v1/workflows/run` | Run a workflow. Body: `Workflow.Definition`. Returns the Workflow Result with per-step outcomes. |

## Built-in tools

| Tool | Inputs | Output |
|------|--------|--------|
| `http` | `method`, `url`, `body?`, `headers?` | `body`, `status_code` |
| `shell` | `command`, `env?`, `timeout_sec?` | `stdout`, `stderr`, `exit_code` |
| `json_transform` | `input`, `expr` | the value of `expr` evaluated against `input` |
| `prompt_call` | `release_id`, `inputs?` | the LLM's `output` field |

Tool invocations are sandboxed:

- The `shell` tool runs in a scrubbed environment — the
  daemon's process env is filtered to a `PROMPTSHEON_*`
  allowlist before exec.
- The `shell` tool times out via `context.WithTimeout`; the
  process group is killed on timeout (no forked children
  outlive the cancellation).
- The `http` tool refuses non-HTTPS to private / loopback
  addresses — same SSRF rules as webhooks.

## Operator guide

The `internal/workflow` package is the Workflow runtime. It
is gated behind `PROMPTSHEON_HARNESS_PRECONDITIONS=true` —
precondition execution is off by default so an unconfigured
Workflow never runs any step.

```bash
# Enable preconditions + workflow execution.
export PROMPTSHEON_HARNESS_PRECONDITIONS=true
./promptsheond
```

Production tenants that want full DAG execution (DAG with
`depends_on` between Steps, parallel branches, error
retries) ship that as a follow-on. Today's `Step` is a
linear sequence; the Workflow's `steps` field is the
authoritative order.