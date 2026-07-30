# CLI Reference

The `promptsheon` CLI is a hand-rolled command dispatcher under
`cli.go`. It talks to a running daemon over
HTTP — it is **not** a standalone binary. Start the daemon
first (`./promptsheond`), then run CLI commands.

## Invocation

```bash
promptsheon <command> [subcommand] [args]
promptsheon --version | --help
```

The CLI talks to the daemon over loopback HTTP. The base URL
comes from the `PROMPTSHEON_SERVER` environment variable
(default `http://localhost:8080`). The CLI does **not** send
an `Authorization` header today; for an authenticated daemon
the CLI must be run against a loopback listener or wrapped by
a tool that injects the API key. The CLI's HTTP helpers
refuse to talk to non-loopback hosts (`validateLocalURL` in
`cli_http.go`).

`--version` (also `-version`, `-V`) prints the build info.
`--help` (also `-help`, `-h`) prints the usage block.

## Commands

The actual handler surface is registered in
`cli.go`'s `commandHandlers` map. Top-level
commands:

| Command | Description |
|---------|-------------|
| `init` | Initialize a local CAS repository (`.promptsheon/`). |
| `hash-object <data>` | Compute SHA-256 of `<data>`. |
| `write-object <data>` | Write a blob to the local CAS. |
| `read-object <hash>` | Read + pretty-print a CAS object. |
| `commit <tree> [msg]` | Create a commit on the current branch. Reads `PROMPTSHEON_AUTHOR` and `PROMPTSHEON_TELEMETRY`. |
| `log [n]` | Show commit history (newest first). |
| `checkout <ref\|hash>` | Switch to a branch or commit. |
| `branch [name] [hash]` | Create or list branches. |
| `delete-branch <name>` | Delete a branch. |
| `diff <hashA> <hashB>` | Diff two states. |
| `status` | Repository state summary. |
| `show <hash>` | Object details (type-aware). |
| `ls-tree <hash>` | List tree entries. |
| `cat-file <hash>` | Output blob content to stdout. |
| `graph` | Commit DAG visualization. |
| `stats` | Repository statistics. |
| `verify` | Check repository integrity. |
| `run --provider <p> --model <m> --prompt <text>` | Run a single prompt through the LLM gateway. |
| `provider list` | List registered LLM providers. |
| `provider test <name>` | Smoke-test a provider. |
| `workspace list\|create\|get\|delete` | Workspace CRUD against the API. |
| `project list <ws_id>\|create <ws_id> <name>\|get <id>\|delete <id>` | Project CRUD. |
| `capability list <proj_id>\|create <proj_id> <name>\|get <id>\|delete <id>` | Capability CRUD. |
| `release list <cap_id>\|create <ver_id> <env>\|get <id>\|vote <id> <identity> <approve\|reject\|abstain>\|activate <id>\|rollback <id>\|invoke <id> --model <m>\|approval <id>` | Release lifecycle. |
| `dataset list <cap_id>\|create <cap_id> --name <n> [--file cases.json]\|get <id>\|put-cases <id> <file>\|delete <id>` | Dataset CRUD. |
| `precondition list <cap_id>\|add <cap_id> --name <n> --cmd <c> [--timeout N]\|delete <id>` | Precondition CRUD. |
| `eval list <rel_id>\|run <rel_id> --dataset <id> [--scorer <name>]\|get <id>` | Eval runner. |
| `selfevolve enable <cap_id> [--dataset <id>] [--min-score N] [--max-revisions N] [--cooldown-sec N] [--target-env env]\|disable <cap_id>\|status <cap_id>` | Toggle closed-loop self-evolution on a Capability. |

## Examples

```bash
# Drive a Release lifecycle against a running daemon.
REL=$(promptsheon release create v1 prod | jq -r .id)
promptsheon release vote $REL bob approve
promptsheon release activate $REL
promptsheon release invoke $REL --model claude-haiku-4-5

# Wire the harness loop.
promptsheon dataset create c1 --name greeting --file cases.json
promptsheon precondition add c1 --name go-test --cmd "go test ./..." --timeout 60
promptsheon eval run $REL --dataset <dataset_id> --scorer exact_match

# Enable closed-loop self-evolve.
promptsheon selfevolve enable c1 --dataset <ds_id> --min-score 0.9 \
  --max-revisions 10 --cooldown-sec 900 --target-env dev
promptsheon selfevolve status c1
```

## Local-only HTTP client

The CLI's `httpGet` / `httpPost` / `httpDelete` helpers in
`cli_http.go` refuse any non-loopback host. The
CLI is a thin wrapper over the daemon's REST API; anything
beyond this surface goes through the [Go](../reference/sdk.md) /
[Python](../reference/sdk.md) / [TypeScript](../reference/sdk.md) SDKs instead.