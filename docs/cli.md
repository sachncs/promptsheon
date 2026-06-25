# CLI

The `promptsheon` binary is a Git-style client for a local `.promptsheon/` repository. It does not talk to the server. For server-side operations (creating prompts, running agents, evaluations) use the HTTP API or the [SDK](sdk.md).

```bash
promptsheon --help
```

## Repository commands

| Command | What it does |
|---|---|
| `init` | Initialize a new `.promptsheon/` directory in the current working directory. Idempotent: errors if already initialized. |
| `status` | Show a one-line summary of the current branch, the head commit, and the working state. |
| `log [n]` | Show the last `n` commit hashes and messages (default `10`). |
| `graph` | Render the commit DAG as ASCII art. Useful for understanding branches. |
| `stats` | Print repository counts (objects, refs, total bytes on disk). |
| `verify` | Walk every object and confirm that each one's on-disk content hashes to its filename. Catches disk corruption. |

## Object commands

The CLI is content-addressable. You can write a blob and read it back by hash.

| Command | What it does |
|---|---|
| `hash-object <data>` | Compute the SHA-256 hash of a blob. Does **not** write it. |
| `write-object <data>` | Compute the hash, write the blob to `.promptsheon/objects/`, print the hash. |
| `read-object <hash>` | Read an object from the store. |
| `cat-file <hash>` | Output a blob's content to stdout. |
| `ls-tree <hash>` | List the entries of a tree object. |
| `show <hash>` | Pretty-print any object (blob / tree / commit). |

## Branch and ref commands

| Command | What it does |
|---|---|
| `branch` | List all branches. The current one is marked `*`. |
| `branch <name> [hash]` | Create a branch pointing at `hash` (or the current head). |
| `delete-branch <name>` | Delete a branch. Refuses to delete the current branch. |
| `checkout <ref\|hash>` | Switch the HEAD to a branch or a detached commit. |
| `commit <tree> [msg]` | Create a commit on the current branch. `tree` is the hash of a tree object. |

## Diff and inspection

| Command | What it does |
|---|---|
| `diff <hashA> <hashB>` | Diff two states. Reports prompt content, telemetry, and any structural changes. |
| `diff` (no args) | Diff the working tree against the current head. |

## Server-side helpers

These commands talk to the configured LLM provider via environment variables. They do **not** require the server to be running.

| Command | What it does |
|---|---|
| `run --provider <p> --model <m> --prompt <text>` | Run a one-shot prompt against an LLM provider. Useful for testing connectivity and the fallback chain. |
| `provider list` | List registered providers (those with credentials in the environment). |
| `provider test <name>` | Send a tiny test prompt and report latency. |

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success. |
| `1` | Command error or unknown command. |
| non-zero | Reserved for future use. |

The error message is written to stderr in the form `error: <message>`.

## Examples

### Bootstrap a repository

```bash
mkdir my-agent && cd my-agent
promptsheon init
# initialized empty promptsheon repository in .promptsheon/
```

### Stage a prompt and commit

```bash
PROMPT='You are a concise assistant. Answer: {{question}}'
HASH=$(promptsheon write-object "$PROMPT")
TREE=$(promptsheon commit-tree $HASH)   # pseudo; use the helper you have
promptsheon commit $TREE "initial prompt"
```

### Inspect the DAG

```bash
promptsheon log
# a1b2c3d  initial prompt
# 9f8e7d6  add evaluation step

promptsheon graph
```

### Verify integrity

```bash
promptsheon verify
# 1342 objects checked, 0 errors
```

## Environment

| Variable | Purpose |
|---|---|
| `PROMPTSHEON_AUTHOR` | Default author name and email for commits. |
| `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, etc. | Provider credentials for `run` and `provider test`. |
| `PROMPTSHEON_OLLAMA_BASE_URL` | Override the default Ollama URL (`http://localhost:11434`). |

The CLI does **not** read `PROMPTSHEON_DB_PATH`, `PROMPTSHEON_ADDR`, or any of the server-only env vars. It operates on a local directory.

## See also

- [Architecture](architecture.md) — the object model
- [Algorithms — CAS](algorithms.md#cas-content-addressable-storage) (in the broader algorithms page)
- ADR [0001](adr/0001-use-cas-for-prompt-history.md) — why CAS
