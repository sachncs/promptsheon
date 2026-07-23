# CLI Reference

The `promptsheon` CLI is a hand-rolled command dispatcher under
`cmd/promptsheon/main.go`. It talks to a running daemon over
HTTP — it is **not** a standalone binary. Start the daemon
first (`./promptsheond`), then run CLI commands.

## Invocation

```bash
promptsheon [global flags] <command> [subcommand] [args]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--base-url` | `http://127.0.0.1:8080` | Daemon base URL. |
| `--api-key` | (none) | API key for `Authorization: Bearer`. Falls back to `PROMPTSHEON_API_KEY`. |
| `--json` | `false` | Emit machine-readable JSON instead of the default human output. |
| `--help` | | Show per-command help. |

## Commands

| Command | Description |
|---------|-------------|
| `promptsheon workspace list` | List Workspaces. |
| `promptsheon workspace get <id>` | Get one Workspace. |
| `promptsheon workspace create <name>` | Create a Workspace. |
| `promptsheon project list <workspace_id>` | List Projects under a Workspace. |
| `promptsheon project create <workspace_id> <name>` | Create a Project. |
| `promptsheon capability list <project_id>` | List Capabilities under a Project. |
| `promptsheon capability get <id>` | Get one Capability. |
| `promptsheon capability create <project_id> <name>` | Create a Capability. |
| `promptsheon version list <capability_id>` | List Versions. |
| `promptsheon version get <id>` | Get one Version. |
| `promptsheon version add <capability_id> <manifest.json>` | Add a Version from a manifest JSON file. |
| `promptsheon release create <version_id> '<json>'` | Create a Pending Release. JSON: `{"environment":"prod"}`. |
| `promptsheon release vote <release_id> <identity> <decision>` | Cast a vote. `<decision>` is `approve` or `reject`. |
| `promptsheon release activate <release_id>` | Transition Pending → Active. 409 if MakerChecker quorum not satisfied. |
| `promptsheon release rollback <release_id>` | Transition Active → RolledBack. |
| `promptsheon release approval <release_id>` | Show the vote trail. |
| `promptsheon release invoke <release_id> '<inputs-json>'` | Invoke a Release. JSON: `{"inputs": {...}}`. |
| `promptsheon dataset create <capability_id> <name> [cases.json]` | Create a Dataset. |
| `promptsheon dataset list <capability_id>` | List Datasets. |
| `promptsheon dataset get <id>` | Get one Dataset (with cases). |
| `promptsheon dataset put-cases <dataset_id> [cases.json]` | Replace cases atomically. |
| `promptsheon dataset delete <id>` | Delete a Dataset. |
| `promptsheon precondition add <capability_id> <name> <command> [timeout_sec]` | Add a Precondition. |
| `promptsheon precondition list <capability_id>` | List Preconditions. |
| `promptsheon precondition update <id> '<json>'` | Update a Precondition (partial fields). |
| `promptsheon precondition delete <id>` | Delete a Precondition. |
| `promptsheon eval run <release_id> <dataset_id> [scorer]` | Run an Eval. `<scorer>` is `exact_match`, `contains`, `regex`, or `json_schema`. |
| `promptsheon eval list <release_id>` | List EvalRuns for a Release. |
| `promptsheon eval get <id>` | Get an EvalRun with per-case results. |
| `promptsheon provider list` | List registered LLM provider names. |
| `promptsheon provider test <name> --model <model>` | Smoke-test a provider. `--model` is required. |
| `promptsheon webhook list` | List webhook endpoints. |
| `promptsheon webhook add '<json>'` | Register a webhook. JSON: `{"url":"https://...","events":["eval.completed",...],"secret?":"..."}`. |
| `promptsheon webhook delete <id>` | Remove a webhook. |
| `promptsheon vault add '<json>'` | Save a provider key. JSON: `{"provider_name":"openai","key_name":"prod","key":"sk-..."}`. |
| `promptsheon vault list` | List provider keys. |
| `promptsheon vault delete <id>` | Delete a provider key. |
| `promptsheon user list` | List users. |
| `promptsheon user create '<json>'` | Create a user. |
| `promptsheon user get <id>` | Get a user. |
| `promptsheon user update <id> '<json>'` | Update a user. |
| `promptsheon user delete <id>` | Delete a user. |
| `promptsheon alert list-rules` | List alert rules. |
| `promptsheon alert add-rule '<json>'` | Create an alert rule. |
| `promptsheon alert list-active` | List active (firing) alerts. |
| `promptsheon alert resolve <id>` | Resolve an active alert. |
| `promptsheon audit list [filters]` | List audit entries. Filters: `--user-id`, `--resource`, `--action`, `--since`, `--until`, `--limit`. |
| `promptsheon audit verify` | Verify the chain. |
| `promptsheon health` | Daemon liveness. |
| `promptsheon version` | Daemon build version. |

## Examples

```bash
# 1. Health probe.
promptsheon health
# healthy

# 2. Drive a full Release lifecycle.
promptsheon workspace create acme
promptsheon project create w1 summariser
promptsheon capability create p1 summariser
promptsheon version add c1 manifest.json
REL=$(promptsheon release create v1 '{"environment":"prod"}' | jq -r .id)
promptsheon release vote $REL bob approve
promptsheon release activate $REL
promptsheon release invoke $REL '{"q":"hello"}'

# 3. Run an Eval.
promptsheon dataset create c1 greeting cases.json
promptsheon eval run $REL <dataset_id> exact_match

# 4. Audit chain.
promptsheon audit verify
# {"ok":true, "last_row_id":42, "last_hash":"..."}
```

## Scripting

Every command supports `--json` for machine-readable output.
Combine with `jq` for ad-hoc filtering:

```bash
promptsheon --json capability list p1 | jq '.[] | {id, name}'
```