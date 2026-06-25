# Workflows

Promptsheon's workflow engine executes multi-step agent pipelines as a Directed Acyclic Graph (DAG). Steps with no dependencies run in parallel; dependent steps wait for their inputs.

## Concepts

### Agent

An agent is a named collection of steps and tool references. Each step references a prompt and declares which other steps it depends on.

### Step

A single unit of work in a workflow:

```json
{
  "id": "summarize",
  "prompt_id": "prompt-abc123",
  "depends_on": ["research"],
  "tool_calls": [],
  "output_key": "summary",
  "condition": {
    "field": "research.quality",
    "operator": "gt",
    "value": "0.5"
  }
}
```

| Field | Description |
|---|---|
| `id` | Unique step identifier |
| `prompt_id` | Prompt to execute (or use tool_calls instead) |
| `depends_on` | Step IDs that must complete before this step runs |
| `tool_calls` | Tool invocations to execute |
| `output_key` | Key under which this step's output is stored |
| `condition` | Optional branching condition |

### Tool Calls

Steps can invoke registered tools (HTTP, shell, JSON transform, prompt call):

```json
{
  "tool": "search",
  "input": {"query": "{{input.topic}}"}
}
```

Tool output is stored under the step's `output_key` or the tool name.

### Conditions

Branching conditions allow steps to be skipped based on previous outputs:

```json
{
  "field": "research.confidence",
  "operator": "gt",
  "value": "0.7"
}
```

Supported operators: `eq`, `neq`, `contains`, `gt`, `lt`, `exists`.

### Template Variables

Tool inputs support template interpolation:

- `{{input.xxx}}` — reference workflow input
- `{{step_context.yyy.zzz}}` — reference previous step output

## Execution Model

1. **DAG validation** — cycles are detected and rejected before execution. The validator uses a depth-first search and rejects on the first back-edge.
2. **Topological sort** — steps are grouped into levels of independent steps (Kahn's algorithm). A step with no dependencies is on level 0; a step whose dependencies are all on level ≤ N-1 is on level N.
3. **Level-by-level execution** — all steps in a level run concurrently. The next level starts only when the previous level is fully resolved.
4. **Failure propagation** — if a step fails, all transitive descendants are marked `skipped` and never start.
5. **Cancellation** — a cancelled `context.Context` is honoured by every step. In-flight steps are given a chance to clean up, then marked `cancelled`.

See [Algorithms — Workflow DAG execution](algorithms.md#workflow-dag-execution) and ADR [0008](adr/0008-workflow-dag-with-topological-execution.md).

### Status Values

| Status | Description |
|---|---|
| `pending` | Waiting for dependencies |
| `running` | Currently executing |
| `completed` | Finished successfully |
| `failed` | Encountered an error |
| `skipped` | Dependency failed or condition not met |
| `cancelled` | Workflow was cancelled |

## API Usage

### Execute a Workflow

```bash
curl -X POST http://localhost:8080/api/v1/agents/{id}/execute \
  -H "Content-Type: application/json" \
  -d '{
    "input": {"topic": "quantum computing", "style": "technical"},
    "provider": "openai",
    "model": "gpt-4o"
  }'
```

### Run a Standalone Workflow

```bash
curl -X POST http://localhost:8080/api/v1/workflows/run \
  -H "Content-Type: application/json" \
  -d '{
    "steps": [
      {"id": "step1", "prompt_id": "p1", "depends_on": []},
      {"id": "step2", "prompt_id": "p2", "depends_on": ["step1"]}
    ],
    "input": {"query": "hello"}
  }'
```

### Validate a Workflow

```bash
curl -X POST http://localhost:8080/api/v1/agents/validate \
  -H "Content-Type: application/json" \
  -d '{
    "steps": [
      {"id": "a", "depends_on": ["b"]},
      {"id": "b", "depends_on": ["a"]}
    ]
  }'
```

Returns an error if cycles are detected.

### Cancel a Running Workflow

```bash
curl -X PUT http://localhost:8080/api/v1/workflows/{id}/cancel
```

## Tools

Register tools in the tool registry before execution. Built-in tool types:

| Type | Description |
|---|---|
| `http` | Make HTTP requests to external services |
| `shell` | Execute shell commands. **Disabled by default.** See policy below. |
| `json_transform` | Transform JSON data between steps |
| `prompt_call` | Call another prompt as a sub-workflow |

### Shell tool policy

The `shell` tool can execute arbitrary commands. It is therefore disabled by default and the policy is **fail-closed**:

- `PROMPTSHEON_SHELL_ENABLED=true` enables the tool only if `PROMPTSHEON_SHELL_ALLOWLIST` contains at least one command.
- An empty allowlist with the tool "enabled" is coerced to disabled (the server logs a warning and forces the enabled flag to `false`).
- The allowlist matches the **first token** of the command (the program name). Arguments are not constrained.

```bash
# Enable only `ls` and `cat` (no arguments constrained)
PROMPTSHEON_SHELL_ENABLED=true
PROMPTSHEON_SHELL_ALLOWLIST=ls,cat
```

See [Security](security.md#shell-tool-policy) for the rationale.
